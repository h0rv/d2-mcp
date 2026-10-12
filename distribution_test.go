package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type clientCommand struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func packageJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func TestStaticClientPackages(t *testing.T) {
	const plugin = "agents/package"
	want := clientCommand{"stdio", "d2-mcp", []string{"--transport", "stdio"}}
	for _, name := range []string{"mcp.json", ".mcp.json"} {
		var config struct{ MCPServers map[string]clientCommand }
		packageJSON(t, filepath.Join(plugin, name), &config)
		if len(config.MCPServers) != 1 || !reflect.DeepEqual(config.MCPServers["d2"], want) {
			t.Fatalf("%s: unexpected command: %+v", name, config)
		}
	}
	for _, name := range []string{".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
		var manifest map[string]any
		packageJSON(t, filepath.Join(plugin, name), &manifest)
		if manifest["name"] != "d2" || manifest["hooks"] != nil {
			t.Fatalf("unexpected plugin identity or executable hooks: %s", name)
		}
		for _, key := range []string{"commands", "skills", "mcpServers"} {
			if path, ok := manifest[key].(string); ok {
				if _, err := os.Stat(filepath.Join(plugin, path)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	for _, path := range []string{".claude-plugin/marketplace.json", ".agents/plugins/marketplace.json"} {
		var catalog struct {
			Name    string
			Plugins []struct {
				Name   string
				Source json.RawMessage
			}
		}
		packageJSON(t, path, &catalog)
		if catalog.Name != "d2-mcp" || len(catalog.Plugins) != 1 || catalog.Plugins[0].Name != "d2" {
			t.Fatalf("unexpected marketplace: %s", path)
		}
		var source string
		if err := json.Unmarshal(catalog.Plugins[0].Source, &source); err != nil {
			var local struct{ Source, Path string }
			if err := json.Unmarshal(catalog.Plugins[0].Source, &local); err != nil || local.Source != "local" {
				t.Fatalf("invalid marketplace source: %s", path)
			}
			source = local.Path
		}
		if filepath.Clean(source) != filepath.FromSlash(plugin) {
			t.Fatalf("marketplace %s points outside the shared package", path)
		}
	}
	for _, path := range []string{"package.json", plugin + "/package.json"} {
		var manifest struct {
			Private               bool
			Scripts, Dependencies map[string]any
			Pi                    struct{ Skills, Prompts []string }
		}
		packageJSON(t, path, &manifest)
		if !manifest.Private || len(manifest.Scripts) != 0 || len(manifest.Dependencies) != 0 {
			t.Fatalf("Pi metadata must not install executable tooling: %s", path)
		}
		for kind, paths := range map[string][]string{"skills": manifest.Pi.Skills, "prompts": manifest.Pi.Prompts} {
			if len(paths) != 1 || filepath.Join(filepath.Dir(path), paths[0]) != filepath.Join(plugin, kind) {
				t.Fatalf("Pi %s must use the shared %s directory", path, kind)
			}
		}
	}
}

func TestDistribution(t *testing.T) {
	binary := os.Getenv("D2_MCP_TEST_BINARY")
	if binary == "" {
		t.Skip("set D2_MCP_TEST_BINARY to exercise an exact release binary")
	}
	wantVersion := os.Getenv("D2_MCP_TEST_VERSION")
	if wantVersion == "" {
		wantVersion = version
	}
	root := filepath.Join(t.TempDir(), "client workspace 日本語")
	binDir := filepath.Join(root, "installed bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatal(err)
	}
	name := "d2-mcp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(binDir, name)
	source, err := os.Open(binary)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	installed, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(installed, source); err != nil {
		installed.Close()
		t.Fatal(err)
	}
	if err := installed.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(target, "--version")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "d2-mcp "+wantVersion+" (D2 0.9.0)" {
		t.Fatalf("native version: %s (%v)", output, err)
	}
	plugin := filepath.Join(root, "plugin")
	if err := os.CopyFS(plugin, os.DirFS("agents/package")); err != nil {
		t.Fatal(err)
	}
	var config struct{ MCPServers map[string]clientCommand }
	packageJSON(t, filepath.Join(plugin, "mcp.json"), &config)
	server := config.MCPServers["d2"]
	// Real clients resolve the static plugin's command through PATH. Explicit
	// stdio must win even when a surrounding host sets HTTP/SSE defaults.
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MCP_TRANSPORT", "http")
	t.Setenv("SSE_MODE", "true")
	cmd = exec.Command(server.Command, server.Args...)
	cmd.Dir = root
	exerciseClient(t, &mcp.CommandTransport{Command: cmd}, "2025-11-25")

	inputs, outputs := filepath.Join(root, "input diagrams"), filepath.Join(root, "output diagrams")
	for _, dir := range []string{inputs, outputs} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range map[string]string{"main.d2": "...@common.d2\na -> b", "common.d2": "a: ImportedMarker"} {
		if err := os.WriteFile(filepath.Join(inputs, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(root, "outside.svg")
	if err := os.WriteFile(outside, []byte("keep outside file"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(target, "--transport", "stdio", "--input-dir", inputs, "--output-dir", outputs, "--allow-overwrite")
	cmd.Dir = root
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "native-distribution-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if session.InitializeResult().ServerInfo.Version != wantVersion {
		t.Fatal("incorrect MCP version")
	}
	tool := func(name string, args map[string]any, errorCode string) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if errorCode == "" {
			requireOK(t, result)
		} else {
			requireError(t, result, errorCode)
		}
		return result
	}
	tool("compile-d2", map[string]any{"file_path": "main.d2"}, "")
	tool("compile-d2", map[string]any{"file_path": "../outside.svg"}, "invalid_arguments")
	name = "diagram #1 日本語.svg"
	result := tool("render-d2", map[string]any{"file_path": "main.d2", "format": "svg", "output_name": name}, "")
	path := filepath.Join(outputs, name)
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte("ImportedMarker")) {
		t.Fatalf("missing imported artifact: %v", err)
	}
	var rendered struct {
		Artifact struct{ Path, URI, SHA256 string }
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &rendered); err != nil {
		t.Fatal(err)
	}
	if rendered.Artifact.Path != path || rendered.Artifact.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatalf("incorrect artifact metadata: %+v", rendered)
	}
	linked := false
	for _, item := range result.Content {
		if link, ok := item.(*mcp.ResourceLink); ok && link.URI == rendered.Artifact.URI {
			linked = true
		}
	}
	if !linked || !strings.HasPrefix(rendered.Artifact.URI, "file:///") || !strings.Contains(rendered.Artifact.URI, "%23") {
		t.Fatalf("incorrect artifact URI: %s", rendered.Artifact.URI)
	}
	args := map[string]any{"code": "a: ReplacementMarker", "format": "svg", "output_name": name}
	tool("render-d2", args, "write_failed")
	args["overwrite"] = true
	tool("render-d2", args, "")
	data, err = os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte("ReplacementMarker")) {
		t.Fatal("replacement not written")
	}
	args["output_name"] = "../outside.svg"
	tool("render-d2", args, "write_failed")
	data, err = os.ReadFile(outside)
	if err != nil || string(data) != "keep outside file" {
		t.Fatal("outside file changed")
	}
	temps, err := filepath.Glob(filepath.Join(outputs, ".d2-mcp-*"))
	if err != nil || len(temps) != 0 {
		t.Fatal("private staging file left behind")
	}
	result = tool("render-d2", map[string]any{"code": "a: |latex x^2 |\na -> b", "format": "png", "layout": "elk"}, "")
	img, ok := result.Content[0].(*mcp.ImageContent)
	if !ok || img.MIMEType != "image/png" {
		t.Fatal("expected native PNG")
	}
	if _, err := png.Decode(bytes.NewReader(img.Data)); err != nil {
		t.Fatal(err)
	}
}
