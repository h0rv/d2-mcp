package main

import (
	"bytes"
	"context"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/h0rv/d2-mcp/document"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSVGRejectsLaTeXResourceInjection(t *testing.T) {
	a := testApp(t, nil)
	for _, formula := range []string{
		`\color{url(https://example.invalid/paint.svg)}{x}`,
		`\color{uRl(//example.invalid/paint.svg)}{x}`,
		`\color{u\72l(https://example.invalid/paint.svg)}{x}`,
		`\color{url("'#foo'")}{x}`,
		`\color{url('"#foo"')}{x}`,
		`\fcolorbox{red;background-image:image-set("https://example.invalid/p.png" 1x)}{white}{x}`,
		`\fcolorbox{red;--p:url(https://example.invalid/p.png);background-image:var(--p)}{white}{x}`,
	} {
		t.Run(formula, func(t *testing.T) {
			requireError(t, call(t, a, true, map[string]any{"code": "a: |latex\n" + formula + "\n|", "format": "svg"}), "unsupported_feature")
		})
	}
	for _, code := range []string{
		"a: |latex \\color{red}{x^2} |",
		"a.style.fill: \"linear-gradient(90deg, red, blue)\"\na -> b",
		"a -> b: {style.animated: true}",
	} {
		requireOK(t, call(t, a, true, map[string]any{"code": code, "format": "svg", "dark_theme_id": 200}))
	}
}

func TestPNGMathematicalLabels(t *testing.T) {
	a := testApp(t, nil)
	for _, code := range []string{"a: |latex \\color{red}{x^2} |", "a -> b: |latex \\color{red}{x^2} |"} {
		result := call(t, a, true, map[string]any{"code": code, "format": "png"})
		requireOK(t, result)
		image, err := png.Decode(bytes.NewReader(result.Content[0].(*mcp.ImageContent).Data))
		if err != nil || image.Bounds().Empty() {
			t.Fatalf("invalid mathematical PNG: %v", err)
		}
		redPixels := 0
		for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
			for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
				r, g, b, _ := image.At(x, y).RGBA()
				if r > 30000 && r > 2*g && r > 2*b {
					redPixels++
				}
			}
		}
		if redPixels == 0 {
			t.Fatal("formula glyphs were not painted")
		}
	}
	result := call(t, a, true, map[string]any{"code": "a: |latex " + strings.Repeat("x", 4097) + " |", "format": "png"})
	if !result.IsError {
		t.Fatal("oversized formula was admitted")
	}
}

func TestDimensionsIncludeLegendsAndRejectNegativeSizes(t *testing.T) {
	a := testApp(t, nil)
	for _, code := range []string{
		"vars: {d2-legend: {a: " + strings.Repeat("a", 2000) + "}}\nb",
		"a: {width: -1; height: -1}",
	} {
		requireError(t, call(t, a, false, map[string]any{"code": code}), "limit_exceeded")
		for _, format := range []string{"svg", "png"} {
			requireError(t, call(t, a, true, map[string]any{"code": code, "format": format}), "limit_exceeded")
		}
	}
	requireOK(t, call(t, a, true, map[string]any{"code": "vars: {d2-legend: {a: Service}}\nb", "format": "svg"}))
}

func TestArtifactFileURIs(t *testing.T) {
	for path, want := range map[string]string{
		"/tmp/diagram a.svg":           "file:///tmp/diagram%20a.svg",
		"C:/artifacts/diagram a.svg":   "file:///C:/artifacts/diagram%20a.svg",
		"//server/share/diagram.svg":   "file://server/share/diagram.svg",
		"//?/C:/artifacts/diagram.svg": "file:///C:/artifacts/diagram.svg",
		"//?/UNC/server/share/d.svg":   "file://server/share/d.svg",
	} {
		if got := artifactFileURI(path); got != want {
			t.Errorf("URI for %q = %q, want %q", path, got, want)
		}
	}
}

func TestHTTPAdmitsOnlyOneBoundedObject(t *testing.T) {
	const batch = `[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"fetch_d2_cheat_sheet","arguments":{}}}]`
	for _, transport := range []string{"http", "sse"} {
		server := httptest.NewServer(httpHandler(testApp(t, nil).server(), transport))
		t.Cleanup(server.Close)
		check := func(route, version, body string, want int) {
			t.Helper()
			req, _ := http.NewRequest(http.MethodPost, server.URL+route, io.NopCloser(strings.NewReader(body)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("MCP-Protocol-Version", version)
			response, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			data, _ := io.ReadAll(response.Body)
			if response.StatusCode != want || len(data) > 256 {
				t.Fatalf("%s %s %s: %d (%d bytes), want %d", transport, route, version, response.StatusCode, len(data), want)
			}
		}
		for _, body := range []string{batch, `{"jsonrpc":"2.0"}{"jsonrpc":"2.0"}`, "null", "42", "{} garbage"} {
			check("/mcp", "", body, http.StatusBadRequest)
		}
		for _, route := range []string{"/mcp", "/sse?sessionid=untrusted", "/message"} {
			for _, version := range []string{"", "2025-03-26", "2026-07-28"} {
				check(route, version, batch, http.StatusBadRequest)
			}
		}
		check("/mcp", "", strings.Repeat(" ", maxRequestBytes+1), http.StatusRequestEntityTooLarge)
	}
}

type cancellationStore struct{ entered, cancelled chan struct{} }

func (s cancellationStore) Get(ctx context.Context, _ string) (document.Document, error) {
	close(s.entered)
	<-ctx.Done()
	close(s.cancelled)
	return document.Document{}, ctx.Err()
}
func (s cancellationStore) Put(context.Context, string, string, string) (document.Document, error) {
	return document.Document{}, document.ErrConflict
}

func TestCurrentHTTPDisconnectCancelsWork(t *testing.T) {
	store := cancellationStore{make(chan struct{}), make(chan struct{})}
	a := testApp(t, func(c *config) { c.Store = store; c.Timeout = 5 * time.Second })
	server := httptest.NewServer(httpHandler(a.server(), "http"))
	defer server.Close()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}},"name":"compile-d2","arguments":{"document_id":"design","revision":"1"}}}`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", "compile-d2")
	done := make(chan struct{})
	go func() {
		defer close(done)
		response, err := server.Client().Do(req)
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
		}
	}()
	select {
	case <-store.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach backend")
	}
	cancel()
	select {
	case <-store.cancelled:
	case <-time.After(time.Second):
		t.Fatal("disconnected request continued until application timeout")
	}
	<-done
}
