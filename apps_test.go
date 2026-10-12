package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func appCapabilities(settings any) *mcp.ClientCapabilities {
	caps := &mcp.ClientCapabilities{}
	if settings != nil {
		caps.Extensions = map[string]any{diagramUIExtension: settings}
	}
	return caps
}

func supportedAppCapabilities() *mcp.ClientCapabilities {
	return appCapabilities(map[string]any{"mimeTypes": []string{diagramAppMIME}})
}

func connectAppClient(t *testing.T, server *mcp.Server, transport mcp.Transport, caps *mcp.ClientCapabilities, version string) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	if transport == nil {
		var serverTransport mcp.Transport
		transport, serverTransport = mcp.NewInMemoryTransports()
		session, err := server.Connect(ctx, serverTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { session.Close() })
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "apps-test", Version: "1"}, &mcp.ClientOptions{Capabilities: caps})
	session, err := client.Connect(ctx, transport, &mcp.ClientSessionOptions{ProtocolVersion: version})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func checkAppTool(list *mcp.ListToolsResult, wantUI bool) error {
	if len(list.Tools) != 3 {
		return fmt.Errorf("got %d tools; UI must not add tools", len(list.Tools))
	}
	for _, tool := range list.Tools {
		meta, hasUI := tool.Meta["ui"]
		want := wantUI && tool.Name == "render-d2"
		if hasUI != want {
			return fmt.Errorf("%s has UI=%t, want %t", tool.Name, hasUI, want)
		}
		if hasUI {
			ui, ok := meta.(map[string]any)
			if !ok || ui["resourceUri"] != diagramAppURI {
				return fmt.Errorf("invalid UI metadata: %+v", meta)
			}
		}
	}
	return nil
}

func TestAppCapabilityNegotiation(t *testing.T) {
	for _, tc := range []struct {
		name string
		ui   bool
		caps *mcp.ClientCapabilities
		want bool
	}{
		{"disabled", false, supportedAppCapabilities(), false},
		{"supported", true, supportedAppCapabilities(), true},
		{"absent extension", true, appCapabilities(nil), false},
		{"unsupported MIME", true, appCapabilities(map[string]any{"mimeTypes": []string{"text/html"}}), false},
		{"missing MIME", true, appCapabilities(map[string]any{}), false},
		{"invalid MIME", true, appCapabilities(map[string]any{"mimeTypes": true}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp(t, func(c *config) { c.UI = tc.ui })
			session := connectAppClient(t, a.server(), nil, tc.caps, "2025-11-25")
			list, err := session.ListTools(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := checkAppTool(list, tc.want); err != nil {
				t.Fatal(err)
			}
			resources, err := session.ListResources(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.ui && len(resources.Resources) != 1 || !tc.ui && len(resources.Resources) != 0 {
				t.Fatalf("unexpected resources: %+v", resources)
			}
			content, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: diagramAppURI})
			if !tc.ui {
				if err == nil {
					t.Fatal("UI-disabled server exposed the viewer")
				}
				return
			}
			if err != nil || len(content.Contents) != 1 {
				t.Fatalf("read viewer: %+v, %v", content, err)
			}
			resource := content.Contents[0]
			if resource.URI != diagramAppURI || resource.MIMEType != diagramAppMIME || resource.Text != diagramAppHTML || !strings.Contains(strings.ToLower(resource.Text), "<!doctype html>") {
				t.Fatal("resource is not the embedded MCP App")
			}
			for _, meta := range []mcp.Meta{resource.Meta, resources.Resources[0].Meta} {
				ui := meta["ui"].(map[string]any)
				if len(ui) != 2 {
					t.Fatalf("unexpected UI permissions or metadata: %+v", ui)
				}
				permissions, ok := ui["permissions"].(map[string]any)
				if !ok || len(permissions) != 1 || !reflect.DeepEqual(permissions["clipboardWrite"], map[string]any{}) {
					t.Fatalf("only clipboard writes may be requested: %+v", permissions)
				}
				csp := ui["csp"].(map[string]any)
				for _, key := range []string{"connectDomains", "resourceDomains", "frameDomains", "baseUriDomains"} {
					origins, ok := csp[key].([]any)
					if !ok || len(origins) != 0 {
						t.Fatalf("%s must deny external origins, got %+v", key, csp[key])
					}
				}
			}
			if _, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "ui://d2/viewer/../../private"}); err == nil {
				t.Fatal("unexpected resource URI was accepted")
			}
		})
	}
}

func TestAppHTTPMixedClients(t *testing.T) {
	a := testApp(t, func(c *config) { c.UI = true })
	s := httptest.NewServer(httpHandler(a.server(), "http"))
	t.Cleanup(s.Close)
	var clients []*mcp.ClientSession
	for i := range 8 {
		caps := appCapabilities(nil)
		if i%2 == 0 {
			caps = supportedAppCapabilities()
		}
		clients = append(clients, connectAppClient(t, nil, &mcp.StreamableClientTransport{Endpoint: s.URL + "/mcp"}, caps, "2026-07-28"))
	}
	var group sync.WaitGroup
	for i, client := range clients {
		group.Go(func() {
			for range 6 {
				list, err := client.ListTools(t.Context(), nil)
				if err != nil {
					t.Error(err)
					return
				}
				if err := checkAppTool(list, i%2 == 0); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	group.Wait()
}

func TestAppLegacySSECapabilities(t *testing.T) {
	a := testApp(t, func(c *config) { c.UI = true })
	s := httptest.NewServer(httpHandler(a.server(), "sse"))
	t.Cleanup(s.Close)
	for _, supported := range []bool{true, false} {
		caps := appCapabilities(nil)
		if supported {
			caps = supportedAppCapabilities()
		}
		client := connectAppClient(t, nil, &mcp.SSEClientTransport{Endpoint: s.URL + "/sse"}, caps, "2025-11-25")
		list, err := client.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkAppTool(list, supported); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAppStdioHelper(t *testing.T) {
	if os.Getenv("D2_MCP_TEST_APP_STDIO") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("d2-mcp", flag.ContinueOnError)
	os.Args = []string{"d2-mcp", "--transport", "stdio", "--ui"}
	if err := run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestAppStdioProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestAppStdioHelper$")
	cmd.Env = append(os.Environ(), "D2_MCP_TEST_APP_STDIO=1")
	client := connectAppClient(t, nil, &mcp.CommandTransport{Command: cmd}, supportedAppCapabilities(), "2025-11-25")
	list, err := client.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkAppTool(list, true); err != nil {
		t.Fatal(err)
	}
	resource, err := client.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: diagramAppURI})
	if err != nil || len(resource.Contents) != 1 || resource.Contents[0].Text != diagramAppHTML {
		t.Fatalf("CLI --ui did not serve the embedded viewer: %v", err)
	}
}

func TestAppLegacyStatelessHTTP(t *testing.T) {
	a := testApp(t, func(c *config) { c.UI = true })
	s := httptest.NewServer(httpHandler(a.server(), "http"))
	t.Cleanup(s.Close)
	// These handshakes are deliberately different. Legacy stateless HTTP cannot
	// retain either one; metadata must not depend on another client's last init.
	for _, caps := range []*mcp.ClientCapabilities{supportedAppCapabilities(), appCapabilities(nil)} {
		client := connectAppClient(t, nil, &mcp.StreamableClientTransport{Endpoint: s.URL + "/mcp"}, caps, "2025-11-25")
		list, err := client.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkAppTool(list, true); err != nil {
			t.Fatal(err)
		}
		// When a legacy client supplies explicit capabilities on this request,
		// honor them instead of applying the unknown-capability exception.
		for _, explicit := range []any{map[string]any{}, nil, "invalid"} {
			list, err = client.ListTools(t.Context(), &mcp.ListToolsParams{Meta: mcp.Meta{
				mcp.MetaKeyClientCapabilities: explicit,
			}})
			if err != nil {
				t.Fatal(err)
			}
			if err := checkAppTool(list, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, version := range []string{"2025-11-25", "2026-07-28"} {
		params := map[string]any{}
		if version == "2026-07-28" {
			// Current protocol requires capabilities; do not treat malformed
			// current requests as legacy capability-unknown requests.
			params["_meta"] = map[string]any{mcp.MetaKeyProtocolVersion: version, mcp.MetaKeyClientInfo: map[string]any{"name": "test", "version": "1"}}
		}
		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": params})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(http.MethodPost, s.URL+"/mcp", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", version)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var wire struct {
			Result *mcp.ListToolsResult `json:"result"`
			Error  *json.RawMessage     `json:"error"`
		}
		err = json.NewDecoder(response.Body).Decode(&wire)
		response.Body.Close()
		if err != nil || response.Header.Get("Mcp-Session-Id") != "" {
			t.Fatalf("stateless response: %v, %+v", err, response.Header)
		}
		if version == "2026-07-28" {
			if wire.Error == nil || wire.Result != nil {
				t.Fatal("missing current-protocol capabilities accepted")
			}
		} else if wire.Result == nil || checkAppTool(wire.Result, true) != nil {
			t.Fatalf("legacy stateless discovery without init failed: %+v", wire)
		}
	}
}

func TestAppPreservesOrdinaryResults(t *testing.T) {
	plain := testApp(t, nil)
	plainServer := httptest.NewServer(httpHandler(plain.server(), "http"))
	t.Cleanup(plainServer.Close)
	plainClient := connectAppClient(t, nil, &mcp.StreamableClientTransport{Endpoint: plainServer.URL + "/mcp"}, supportedAppCapabilities(), "2026-07-28")
	a := testApp(t, func(c *config) { c.UI = true })
	s := httptest.NewServer(httpHandler(a.server(), "http"))
	t.Cleanup(s.Close)
	client := connectAppClient(t, nil, &mcp.StreamableClientTransport{Endpoint: s.URL + "/mcp"}, supportedAppCapabilities(), "2026-07-28")
	for _, args := range []map[string]any{
		{"code": "first -> second", "format": "ascii"},
		{"code": "a: {", "format": "svg"},
		{"code": "a -> b", "format": "svg"},
		{"code": "replacement -> preview", "format": "png"},
		{"code": "a -> b", "format": "svg", "unknown": true},
	} {
		got, err := client.CallTool(t.Context(), &mcp.CallToolParams{Name: "render-d2", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		want, err := plainClient.CallTool(t.Context(), &mcp.CallToolParams{Name: "render-d2", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		gotJSON, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		wantJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		// HTTP current-protocol results include SDK response _meta. Compare the
		// meaningful content and structured artifacts after removing that field.
		var gotWire, wantWire map[string]any
		if err := json.Unmarshal(gotJSON, &gotWire); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(wantJSON, &wantWire); err != nil {
			t.Fatal(err)
		}
		delete(gotWire, "_meta")
		delete(wantWire, "_meta")
		if !reflect.DeepEqual(gotWire, wantWire) {
			t.Fatalf("UI changed ordinary result for %+v", args)
		}
	}
}
