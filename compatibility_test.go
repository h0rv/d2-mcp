package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBoardSelectionAndInheritance(t *testing.T) {
	a := testApp(t, nil)
	code := `root: RootMarker
layers: {isolated: {child: LayerMarker; scenarios: {zoom: {child.label: NestedMarker}}}}
scenarios: {variant: {root.label: ScenarioMarker}}
steps: {one: {first: FirstMarker}; two: {second: SecondMarker}}`
	for _, tc := range []struct {
		path         []string
		want, absent string
	}{
		{nil, "RootMarker", "LayerMarker"},
		{[]string{"layers", "isolated"}, "LayerMarker", "RootMarker"},
		{[]string{"layers", "isolated", "scenarios", "zoom"}, "NestedMarker", "RootMarker"},
		{[]string{"scenarios", "variant"}, "ScenarioMarker", "LayerMarker"},
		{[]string{"steps", "two"}, "FirstMarker", "LayerMarker"},
	} {
		for _, format := range []string{"svg", "png", "ascii"} {
			r := call(t, a, true, map[string]any{"code": code, "board_path": tc.path, "format": format})
			requireOK(t, r)
			if format == "svg" {
				svg := r.Content[0].(*mcp.EmbeddedResource).Resource.Text
				if !strings.Contains(svg, tc.want) || strings.Contains(svg, tc.absent) {
					t.Fatalf("board %v: wrong contents", tc.path)
				}
			}
			if format == "png" {
				if _, err := png.Decode(bytes.NewReader(r.Content[0].(*mcp.ImageContent).Data)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	for _, path := range [][]string{{"layers"}, {"unknown", "x"}, {"layers", "missing"}} {
		requireError(t, call(t, a, true, map[string]any{"code": code, "board_path": path}), "invalid_arguments")
	}
	reserved := `root: Root
layers: {"layers": {scenarios: {missing: {child: Nested}}}}`
	requireError(t, call(t, a, true, map[string]any{"code": reserved, "board_path": []string{"layers", "missing"}}), "invalid_arguments")
	requireOK(t, call(t, a, true, map[string]any{"code": reserved, "board_path": []string{"layers", "layers", "scenarios", "missing"}}))
	tooMany := "layers: {"
	for i := 0; i < 32; i++ {
		tooMany += fmt.Sprintf("b%d: {a};", i)
	}
	requireError(t, call(t, a, false, map[string]any{"code": tooMany + "}"}), "limit_exceeded")
	// The preflight applies to the whole source, including unselected boards.
	requireError(t, call(t, a, false, map[string]any{"code": "root\nlayers: {x: {a.link: javascript:alert(1)}}"}), "unsupported_feature")
}

func TestOptInAssetsAreEmbeddedAndBounded(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	var jpegData, gifData bytes.Buffer
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(&gifData, img, nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"red.png":    encoded.Bytes(),
		"red.jpg":    jpegData.Bytes(),
		"red.gif":    gifData.Bytes(),
		"red.svg":    []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><rect width="20" height="20" fill="red"/></svg>`),
		"active.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><script>alert(1)</script></svg>`),
		"large.svg":  []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="2000" height="20"/>`),
	} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	a := testApp(t, func(c *config) { c.AssetDir = root; c.AllowInlineAssets = true })
	inline := "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
	inlineSVG := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><rect width="20" height="20" fill="red"/></svg>`))
	for _, source := range []string{"red.png", "red.jpg", "red.gif", "red.svg", inline, inlineSVG} {
		for _, format := range []string{"svg", "png"} {
			for _, shape := range []string{"rectangle", "image"} {
				code := fmt.Sprintf("a: {shape: %s; icon: %q; width: 80; height: 80}", shape, source)
				r := call(t, a, true, map[string]any{"code": code, "format": format})
				requireOK(t, r)
				if format == "svg" {
					svg := r.Content[0].(*mcp.EmbeddedResource).Resource.Text
					if !strings.Contains(svg, "data:image/png;base64,") || strings.Contains(svg, "red.svg") || strings.Contains(svg, "red.png") {
						t.Fatal("asset was not embedded")
					}
				} else {
					raster, err := png.Decode(bytes.NewReader(r.Content[0].(*mcp.ImageContent).Data))
					if err != nil {
						t.Fatal(err)
					}
					red := false
					for y := raster.Bounds().Min.Y; y < raster.Bounds().Max.Y; y++ {
						for x := raster.Bounds().Min.X; x < raster.Bounds().Max.X; x++ {
							r, g, b, _ := raster.At(x, y).RGBA()
							if r > 50000 && g < 10000 && b < 10000 {
								red = true
							}
						}
					}
					if !red {
						t.Fatal("PNG did not paint the red asset")
					}
				}
			}
		}
	}
	defaults := testApp(t, nil)
	for _, source := range []string{"red.png", inline} {
		requireError(t, call(t, defaults, true, map[string]any{"code": fmt.Sprintf("a.icon: %q", source)}), "unsupported_feature")
	}
	var hits atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.Write(encoded.Bytes()) }))
	defer remote.Close()
	for _, source := range []string{remote.URL + "/red.png", "../red.png", "file:///tmp/red.png", "active.svg"} {
		requireError(t, call(t, a, true, map[string]any{"code": "a.icon: " + source}), "unsupported_feature")
	}
	if hits.Load() != 0 {
		t.Fatal("asset caused a network request")
	}
	requireError(t, call(t, a, true, map[string]any{"code": "a.icon: large.svg"}), "limit_exceeded")
	secret := filepath.Join(outside, "secret.png")
	if err := os.WriteFile(secret, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "escape.png")); err == nil {
		requireError(t, call(t, a, true, map[string]any{"code": "a.icon: escape.png"}), "unsupported_feature")
	}
	// The SVG importer must not resolve nested external references either.
	external := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><image href="%s/red.png" width="20" height="20"/></svg>`, remote.URL)
	if err := os.WriteFile(filepath.Join(root, "external.svg"), []byte(external), 0600); err != nil {
		t.Fatal(err)
	}
	requireError(t, call(t, a, true, map[string]any{"code": "a.icon: external.svg"}), "unsupported_feature")
	if hits.Load() != 0 {
		t.Fatal("SVG asset caused a network request")
	}
}

func TestAssetExpansionAndFileLimits(t *testing.T) {
	root := t.TempDir()
	a := testApp(t, func(c *config) { c.AssetDir = root })
	noise := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	random := rand.New(rand.NewSource(1))
	for i := 0; i < len(noise.Pix); i += 4 {
		noise.Pix[i] = byte(random.Intn(256))
		noise.Pix[i+1] = byte(random.Intn(256))
		noise.Pix[i+2] = byte(random.Intn(256))
		noise.Pix[i+3] = 255
	}
	var data bytes.Buffer
	if err := png.Encode(&data, noise); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "noise.png"), data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	code := "grid-columns: 5\n"
	for i := 0; i < 20; i++ {
		code += fmt.Sprintf("n%d: {icon: noise.png; width: 80; height: 80}\n", i)
	}
	requireError(t, call(t, a, true, map[string]any{"code": code, "format": "svg"}), "limit_exceeded")
	if err := os.WriteFile(filepath.Join(root, "large.png"), make([]byte, 1<<20+1), 0600); err != nil {
		t.Fatal(err)
	}
	requireError(t, call(t, a, false, map[string]any{"code": "a.icon: large.png"}), "limit_exceeded")
}

func TestExplicitOutputReplacement(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	args := map[string]any{"code": "a: ReplacementMarker", "format": "svg", "output_name": "diagram.svg", "overwrite": true}
	defaults := testApp(t, func(c *config) { c.OutputDir = root })
	requireError(t, call(t, defaults, true, args), "file_access_denied")
	a := testApp(t, func(c *config) { c.OutputDir = root; c.AllowOverwrite = true })
	path := filepath.Join(root, "diagram.svg")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	requireOK(t, call(t, a, true, args))
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte("ReplacementMarker")) {
		t.Fatal("replacement missing", err)
	}
	args["overwrite"] = false
	requireError(t, call(t, a, true, args), "write_failed")
	args["overwrite"] = true
	args["output_name"] = ""
	requireError(t, call(t, a, true, args), "file_access_denied")
	args["output_name"] = "directory.svg"
	if err := os.Mkdir(filepath.Join(root, "directory.svg"), 0700); err != nil {
		t.Fatal(err)
	}
	requireError(t, call(t, a, true, args), "write_failed")
	secret := filepath.Join(outside, "secret.svg")
	if err := os.WriteFile(secret, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "link.svg")); err == nil {
		args["output_name"] = "link.svg"
		requireError(t, call(t, a, true, args), "write_failed")
	}
	if err := os.Link(secret, filepath.Join(root, "hardlink.svg")); err == nil {
		args["output_name"] = "hardlink.svg"
		requireOK(t, call(t, a, true, args))
	}
	data, err = os.ReadFile(secret)
	if err != nil || string(data) != "keep" {
		t.Fatal("outside file modified", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".d2-mcp-") {
			t.Fatal("temporary artifact leaked")
		}
	}
}

func TestCompatibilityOptionsOverHTTP(t *testing.T) {
	root, output := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "icon.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10" fill="red"/></svg>`), 0600); err != nil {
		t.Fatal(err)
	}
	a := testApp(t, func(c *config) { c.AssetDir = root; c.OutputDir = output; c.AllowOverwrite = true })
	host := httptest.NewServer(httpHandler(a.server(), "http"))
	defer host.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "compatibility-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: host.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range list.Tools {
		if tool.Name == "render-d2" && (tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint) {
			t.Fatal("overwrite capability not annotated")
		}
	}
	for _, marker := range []string{"FirstMarker", "SecondMarker"} {
		code := fmt.Sprintf("layers: {child: {a: {label: %s; icon: icon.svg}}}", marker)
		r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "render-d2", Arguments: map[string]any{"code": code, "board_path": []string{"layers", "child"}, "format": "svg", "output_name": "current.svg", "overwrite": true}})
		if err != nil {
			t.Fatal(err)
		}
		if r.IsError {
			t.Fatalf("call failed: %+v", r)
		}
		data, err := os.ReadFile(filepath.Join(output, "current.svg"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data, []byte(marker)) || !bytes.Contains(data, []byte("data:image/png;base64,")) {
			t.Fatal("file pipeline did not preserve board, asset, or replacement")
		}
	}
}
