package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Test that the Apps handler returns structured payload + widget meta so ChatGPT can sandbox the viewer.
func TestRenderD2AppsHandler_AsciiPayloadAndMeta(t *testing.T) {
	// Minimal server globals the handler depends on.
	GlobalRenderFormat = "ascii"
	GlobalASCIIMode = "extended"
	GlobalAppsSDKEnabled = true
	supportedFormats = []string{"ascii", "svg"}
	supportedFormatSet = map[string]struct{}{"ascii": {}, "svg": {}}

	// Create request with arguments as JSON
	argsJSON, _ := json.Marshal(map[string]any{
		"code":   "a -> b: test",
		"format": "ascii",
	})

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "render_d2",
			Arguments: argsJSON,
		},
	}

	result, err := RenderD2AppsHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Structured content should be the diagram payload with ASCII populated and live flag set.
	payloadJSON, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var payload appsDiagramPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Code == "" {
		t.Fatalf("payload code missing")
	}
	if payload.Format != "ascii" {
		t.Fatalf("payload format = %s, want ascii", payload.Format)
	}
	if payload.ASCII == "" {
		t.Fatalf("payload ascii missing")
	}
	if !payload.Live {
		t.Fatalf("payload live flag should be true")
	}

	// Response should contain text content (model narration)
	if len(result.Content) == 0 {
		t.Fatal("expected content array to have at least one item")
	}

	var foundText bool
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			foundText = true
			if tc.Text == "" {
				t.Error("text content should not be empty")
			}
		}
	}
	if !foundText {
		t.Error("expected at least one TextContent in response")
	}

	// Check that Meta contains the invocation fields
	if result.Meta == nil {
		t.Error("expected Meta to be set on result")
	} else {
		if _, ok := result.Meta["openai/toolInvocation/invoked"]; !ok {
			t.Error("expected openai/toolInvocation/invoked in result Meta")
		}
	}
}
