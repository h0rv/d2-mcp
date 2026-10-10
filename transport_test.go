package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func exerciseClient(t *testing.T, transport mcp.Transport, version string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "integration-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, transport, &mcp.ClientSessionOptions{ProtocolVersion: version})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if session.InitializeResult().ProtocolVersion != version {
		t.Fatalf("negotiated %s instead of %s", session.InitializeResult().ProtocolVersion, version)
	}
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 3 {
		t.Fatalf("tools=%+v", list.Tools)
	}
	for _, tc := range []struct {
		name      string
		args      map[string]any
		wantError bool
	}{
		{"compile-d2", map[string]any{"code": "a -> b"}, false},
		{"compile-d2", map[string]any{"code": "a: {"}, true},
		{"render-d2", map[string]any{"code": "a -> b", "format": "png", "layout": "elk"}, false},
		{"render-d2", map[string]any{"code": "a -> b", "format": "svg", "layout": "tala"}, false},
		{"render-d2", map[string]any{"code": "a -> b", "format": "ascii"}, false},
		{"render-d2", map[string]any{"code": "root\nlayers: {child: {a -> b}}", "board_path": []string{"layers", "child"}, "format": "svg"}, false},
		{"fetch_d2_cheat_sheet", map[string]any{}, false},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if result.IsError != tc.wantError || len(result.Content) == 0 {
			t.Fatalf("%s result: %+v", tc.name, result)
		}
	}
}
func TestStreamableHTTP(t *testing.T) {
	for _, version := range []string{"2025-03-26", "2025-06-18", "2025-11-25", "2026-07-28"} {
		t.Run(version, func(t *testing.T) {
			a := testApp(t, nil)
			server := httptest.NewServer(httpHandler(a.server(), "http"))
			defer server.Close()
			exerciseClient(t, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp"}, version)
		})
	}
}
func TestLegacySSE(t *testing.T) {
	a := testApp(t, nil)
	server := httptest.NewServer(httpHandler(a.server(), "sse"))
	defer server.Close()
	exerciseClient(t, &mcp.SSEClientTransport{Endpoint: server.URL + "/sse"}, "2024-11-05")
}
func TestStdioHelper(t *testing.T) {
	if os.Getenv("D2_MCP_TEST_STDIO") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("d2-mcp", flag.ContinueOnError)
	os.Args = []string{"d2-mcp", "--transport", "stdio"}
	if err := run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
func TestStdioProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestStdioHelper$")
	cmd.Env = append(os.Environ(), "D2_MCP_TEST_STDIO=1")
	exerciseClient(t, &mcp.CommandTransport{Command: cmd}, "2025-11-25")
}
func TestHTTPStatelessAndBoundaries(t *testing.T) {
	a := testApp(t, nil)
	b := testApp(t, nil)
	servers := []*httptest.Server{httptest.NewServer(httpHandler(a.server(), "http")), httptest.NewServer(httpHandler(b.server(), "http"))}
	for _, s := range servers {
		defer s.Close()
	}
	send := func(server *httptest.Server, method, body, origin string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+"/mcp", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-11-25")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { response.Body.Close() })
		if response.Header.Get("Mcp-Session-Id") != "" {
			t.Fatal("stateless server assigned a session")
		}
		return response
	}
	// Initialize on A, then call B without a cookie or session header.
	r := send(servers[0], "POST", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, "")
	if r.StatusCode != 200 {
		t.Fatal(r.Status)
	}
	r = send(servers[1], "POST", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"render-d2","arguments":{"code":"a -> b","format":"svg"}}}`, "")
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != 200 || !bytes.Contains(data, []byte("image/svg+xml")) || bytes.Contains(data, []byte(`"isError":true`)) {
		t.Fatalf("%s: %s", r.Status, data)
	}
	for _, method := range []string{"GET", "DELETE"} {
		if r = send(servers[0], method, "", ""); r.StatusCode != 405 {
			t.Fatalf("%s: %s", method, r.Status)
		}
	}
	if r = send(servers[0], "POST", `{}`, "https://attacker.example"); r.StatusCode != 403 {
		t.Fatal(r.Status)
	}
	if r = send(servers[0], "POST", strings.Repeat(" ", maxRequestBytes+1), ""); r.StatusCode != 413 {
		t.Fatal(r.Status)
	}
	r = send(servers[0], "POST", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"render-d2","arguments":{"code":42}}}`, "")
	var response struct {
		Result struct {
			IsError    bool           `json:"isError"`
			Structured map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.Result.IsError || response.Result.Structured["status"] != "error" {
		t.Fatalf("bad tool error: %+v", response)
	}
}

func TestContainerStdio(t *testing.T) {
	image := os.Getenv("D2_MCP_TEST_IMAGE")
	if image == "" {
		t.Skip("set D2_MCP_TEST_IMAGE to test a locally built container")
	}
	exerciseClient(t, &mcp.CommandTransport{Command: exec.Command("docker", "run", "--rm", "-i", image)}, "2025-11-25")
}
