package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/h0rv/d2-mcp/d2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func testApp(t *testing.T, change func(*config)) *application {
	t.Helper()
	c := config{Format: "png", ASCIIMode: "extended", Timeout: 30 * time.Second}
	if change != nil {
		change(&c)
	}
	a, err := newApplication(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}
func call(t *testing.T, a *application, render bool, args any) *mcp.CallToolResult {
	t.Helper()
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.execute(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: data}}, render)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func requireOK(t *testing.T, r *mcp.CallToolResult) {
	t.Helper()
	if r.IsError {
		t.Fatalf("tool failed: %+v", r.StructuredContent)
	}
}

// Check structured results through their JSON boundary, just as a client does.
func decodeOutput[T any](t *testing.T, r *mcp.CallToolResult) T {
	t.Helper()
	data, err := json.Marshal(r.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output T
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	return output
}

func requireError(t *testing.T, r *mcp.CallToolResult, code string) {
	t.Helper()
	if !r.IsError {
		t.Fatalf("expected error %s", code)
	}
	detail := decodeOutput[errorOutput](t, r).Error
	if code != "" && detail.Code != code {
		t.Fatalf("error=%+v; want %s", detail, code)
	}
	if detail.Message == "" || len(r.Content) == 0 {
		t.Fatal("missing error explanation")
	}
}

func TestRealRenderingLayoutsAndFormats(t *testing.T) {
	a := testApp(t, nil)
	for _, layout := range []string{"dagre", "elk", "tala"} {
		for _, format := range []string{"png", "svg", "ascii"} {
			t.Run(layout+"/"+format, func(t *testing.T) {
				r := call(t, a, true, map[string]any{"code": "direction: right\nclient -> api: request\napi -> db\ndb.shape: cylinder", "layout": layout, "format": format, "sketch": true, "theme_id": 6, "padding": 5})
				requireOK(t, r)
				meta := decodeOutput[renderOutput](t, r).Artifact
				if meta.Bytes == 0 || len(meta.SHA256) != 64 {
					t.Fatalf("bad artifact: %+v", meta)
				}
				switch format {
				case "png":
					img, err := png.Decode(bytes.NewReader(r.Content[0].(*mcp.ImageContent).Data))
					if err != nil {
						t.Fatal(err)
					}
					if img.Bounds().Dx() < 20 || img.Bounds().Dy() < 20 {
						t.Fatal("empty PNG")
					}
				case "svg":
					svg := r.Content[0].(*mcp.EmbeddedResource).Resource.Text
					if !strings.Contains(svg, "<svg") || !strings.Contains(strings.ToLower(svg), "client") {
						t.Fatal("missing SVG labels")
					}
				case "ascii":
					if !strings.Contains(strings.ToLower(r.Content[0].(*mcp.TextContent).Text), "client") {
						t.Fatal("missing ASCII labels")
					}
				}
			})
		}
	}
}
func TestSourceConfigAndExplicitOverrides(t *testing.T) {
	a := testApp(t, nil)
	r := call(t, a, true, map[string]any{"code": "vars: {d2-config: {layout-engine: tala; theme-id: 6; sketch: true; pad: 20}}\na -> b", "format": "svg", "sketch": false, "padding": 0, "theme_id": 0})
	requireOK(t, r)
	o := decodeOutput[renderOutput](t, r).Options
	if o.Layout != "tala" || o.Sketch != false || o.Padding != int64(0) || o.ThemeID != int64(0) {
		t.Fatalf("options=%+v", o)
	}
	// A later call must not inherit mutated D2 option pointers.
	r = call(t, a, false, map[string]any{"code": "a -> b"})
	requireOK(t, r)
	o = decodeOutput[compileOutput](t, r).Options
	if o.Layout != "dagre" || o.Sketch != false || o.Padding != int64(100) {
		t.Fatalf("leaked options=%+v", o)
	}
}
func TestStructuredErrors(t *testing.T) {
	a := testApp(t, nil)
	for _, tc := range []struct {
		name string
		args any
		code string
	}{
		{"missing", map[string]any{}, "invalid_arguments"},
		{"both", map[string]any{"code": "a", "file_path": "a.d2"}, "invalid_arguments"},
		{"wrong type", map[string]any{"code": 42}, "invalid_arguments"},
		{"unknown argument", map[string]any{"code": "a", "unknown": true}, "invalid_arguments"},
		{"fractional padding", map[string]any{"code": "a", "padding": 0.5}, "invalid_arguments"},
		{"syntax", map[string]any{"code": "a: {"}, "invalid_d2"},
		{"semantics", map[string]any{"code": "a.shape: invalid"}, "invalid_d2"},
		{"layout", map[string]any{"code": "a", "layout": "unknown"}, "invalid_arguments"},
		{"source layout", map[string]any{"code": "vars: {d2-config.layout-engine: unknown}\na"}, "invalid_arguments"},
		{"theme", map[string]any{"code": "a", "theme_id": 99999}, "invalid_arguments"},
		{"scale", map[string]any{"code": "a", "scale": 0}, "invalid_arguments"},
		{"source padding", map[string]any{"code": "vars: {d2-config.pad: 99999}\na"}, "invalid_arguments"},
		{"oversized", map[string]any{"code": strings.Repeat("a", d2.MaxSourceBytes+1)}, "invalid_arguments"},
		{"huge dimensions", map[string]any{"code": "a.width: 999999999"}, "limit_exceeded"},
		{"missing board", map[string]any{"code": "a", "board_path": []string{"layers", "missing"}}, "invalid_arguments"},
		{"remote image", map[string]any{"code": "a.icon: http://127.0.0.1:1/private"}, "unsupported_feature"},
		{"local image", map[string]any{"code": "a.icon: /etc/passwd"}, "unsupported_feature"},
		{"unsafe link", map[string]any{"code": "a.link: javascript:alert(1)"}, "unsupported_feature"},
		{"missing input root", map[string]any{"file_path": "a.d2"}, "invalid_arguments"},
		{"ambient import", map[string]any{"code": "...@secret.d2"}, "invalid_d2"},
		{"unconfigured store", map[string]any{"document_id": "a", "revision": "1"}, "invalid_arguments"},
	} {
		t.Run(tc.name, func(t *testing.T) { requireError(t, call(t, a, true, tc.args), tc.code) })
	}
	r := call(t, a, false, map[string]any{"code": "a: {"})
	e := decodeOutput[errorOutput](t, r).Error
	if len(e.Diagnostics) == 0 || e.Diagnostics[0].Line != 1 || e.Diagnostics[0].Column < 1 {
		t.Fatalf("diagnostics=%+v", e)
	}
}
func TestFileImportsAndOutputBoundaries(t *testing.T) {
	input, output, outside := t.TempDir(), t.TempDir(), t.TempDir()
	for name, data := range map[string]string{"main.d2": "...@common.d2\na -> b", "common.d2": "a: Imported"} {
		if err := os.WriteFile(filepath.Join(input, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(outside, "secret.d2")
	if err := os.WriteFile(secret, []byte("secret: Do not read"), 0600); err != nil {
		t.Fatal(err)
	}
	a := testApp(t, func(c *config) { c.InputDir = input; c.OutputDir = output })
	r := call(t, a, true, map[string]any{"file_path": "main.d2", "format": "svg", "output_name": "diagram.svg"})
	requireOK(t, r)
	meta := decodeOutput[renderOutput](t, r).Artifact
	data, err := os.ReadFile(meta.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Imported") {
		t.Fatal("import missing")
	}
	requireError(t, call(t, a, true, map[string]any{"code": "a", "format": "svg", "output_name": "diagram.svg"}), "write_failed")
	for _, name := range []string{"../secret.d2", secret, "."} {
		requireError(t, call(t, a, false, map[string]any{"file_path": name}), "invalid_arguments")
	}
	for _, name := range []string{"../bad.svg", "nested/bad.svg", "bad.png"} {
		requireError(t, call(t, a, true, map[string]any{"code": "a", "format": "svg", "output_name": name}), "write_failed")
	}
	if err := os.Symlink(secret, filepath.Join(input, "escape.d2")); err == nil {
		requireError(t, call(t, a, false, map[string]any{"file_path": "escape.d2"}), "invalid_arguments")
		requireError(t, call(t, a, false, map[string]any{"code": "...@escape.d2"}), "invalid_d2")
	}
	if err := os.Symlink(secret, filepath.Join(output, "escape.svg")); err == nil {
		requireError(t, call(t, a, true, map[string]any{"code": "a", "format": "svg", "output_name": "escape.svg"}), "write_failed")
	}
	after, err := os.ReadFile(secret)
	if err != nil || string(after) != "secret: Do not read" {
		t.Fatal("outside file modified")
	}
}
func TestCancellationAndBusy(t *testing.T) {
	a := testApp(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := a.execute(ctx, &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{"code":"a -> b"}`)}}, true)
	if err != nil {
		t.Fatal(err)
	}
	requireError(t, r, "cancelled")
	a.slots <- struct{}{}
	a.slots <- struct{}{}
	requireError(t, call(t, a, true, map[string]any{"code": "a"}), "busy")
	<-a.slots
	<-a.slots
}
func TestConcurrentRendering(t *testing.T) {
	a := testApp(t, nil)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Go(func() {
			r := call(t, a, true, map[string]any{"code": "a -> b", "format": "svg", "layout": "tala"})
			requireOK(t, r)
		})
	}
	wg.Wait()
}
func TestASCIIModes(t *testing.T) {
	a := testApp(t, nil)
	r := call(t, a, true, map[string]any{"code": "a -> b", "format": "ascii", "ascii_mode": "standard"})
	requireOK(t, r)
	for _, c := range r.Content[0].(*mcp.TextContent).Text {
		if c > 127 {
			t.Fatalf("non ASCII connector: %q", c)
		}
	}
}

func TestBuiltInGrainAndASCIICanvasBudget(t *testing.T) {
	a := testApp(t, nil)
	requireOK(t, call(t, a, true, map[string]any{"code": "a.style.fill-pattern: grain", "format": "svg"}))
	label := strings.Repeat(strings.Repeat("x", 40)+"\n", 60)
	requireError(t, call(t, a, true, map[string]any{"code": "a: |\n" + label + "|", "format": "ascii", "scale": 0.1}), "limit_exceeded")
	for _, attribute := range []string{"icon: https://example.com/a.png", "link: javascript:alert(1)"} {
		requireError(t, call(t, a, false, map[string]any{"code": "vars: {d2-legend: {img: Image {" + attribute + "}}}\na"}), "unsupported_feature")
	}
}

func TestCheatSheetExamples(t *testing.T) {
	a := testApp(t, nil)
	sheet, err := loadCheatSheet()
	if err != nil {
		t.Fatal(err)
	}
	for i, section := range strings.Split(sheet, "```d2\n")[1:] {
		code := strings.SplitN(section, "```", 2)[0]
		t.Run(fmt.Sprint(i), func(t *testing.T) { requireOK(t, call(t, a, true, map[string]any{"code": code, "format": "svg"})) })
	}
}

func TestImportSnapshotAndAggregateBudget(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "common.d2")
	if err := os.WriteFile(path, []byte("a: before"), 0600); err != nil {
		t.Fatal(err)
	}
	a := testApp(t, func(c *config) { c.InputDir = root })
	snapshot := &sourceFS{root: a.input, files: map[string][]byte{}, remaining: d2.MaxSourceBytes}
	first, err := snapshot.Open("common.d2")
	if err != nil {
		t.Fatal(err)
	}
	first.Close()
	if err := os.WriteFile(path, []byte("a: after"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := snapshot.Open("common.d2")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	data, err := io.ReadAll(second)
	if err != nil || string(data) != "a: before" {
		t.Fatalf("snapshot=%s err=%v", data, err)
	}
	snapshot.remaining = 1
	if err := os.WriteFile(filepath.Join(root, "next.d2"), []byte("next"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot.Open("next.d2"); err == nil {
		t.Fatal("aggregate budget ignored")
	}
}

func TestNoAssetNetworkRequests(t *testing.T) {
	var hits atomic.Int64
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.Write([]byte("private")) }))
	defer remote.Close()
	a := testApp(t, nil)
	for _, format := range []string{"svg", "png", "ascii"} {
		requireError(t, call(t, a, true, map[string]any{"code": "a.icon: " + remote.URL + "/asset", "format": format}), "unsupported_feature")
	}
	if hits.Load() != 0 {
		t.Fatal("renderer fetched an asset")
	}
}
