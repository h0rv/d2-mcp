package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Exercise discovery through the SDK: zero bounds, false, and empty objects are
// meaningful JSON Schema values, and must not disappear when using Go structs.
func TestToolSchemaWireValues(t *testing.T) {
	a := testApp(t, func(c *config) { c.Store = &testStore{} })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := a.server().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "schema-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 4 {
		t.Fatalf("got %d tools; want 4", len(list.Tools))
	}
	checkJSON := func(t *testing.T, got json.RawMessage, want string) {
		t.Helper()
		if string(got) != want {
			t.Errorf("got %s; want %s", got, want)
		}
	}
	for _, tool := range list.Tools {
		t.Run(tool.Name, func(t *testing.T) {
			data, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			var schema struct {
				AdditionalProperties json.RawMessage            `json:"additionalProperties"`
				Properties           map[string]json.RawMessage `json:"properties"`
				OneOf                json.RawMessage            `json:"oneOf"`
			}
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatal(err)
			}
			checkJSON(t, schema.AdditionalProperties, `false`)
			switch tool.Name {
			case "fetch_d2_cheat_sheet":
				checkJSON(t, data, `{"additionalProperties":false,"properties":{},"type":"object"}`)
			case "store-d2":
				checkJSON(t, data, `{"additionalProperties":false,"properties":{"code":{"type":"string"},"document_id":{"type":"string"},"expected_revision":{"type":"string"}},"required":["document_id","expected_revision","code"],"type":"object"}`)
			case "compile-d2", "render-d2":
				checkJSON(t, schema.Properties["padding"], `{"maximum":1000,"minimum":0,"type":"integer"}`)
				checkJSON(t, schema.Properties["scale"], `{"maximum":4,"minimum":0.1,"type":"number"}`)
				checkJSON(t, schema.OneOf, `[{"required":["code"]},{"required":["file_path"]},{"required":["document_id","revision"]}]`)
			default:
				t.Fatalf("unexpected tool %q", tool.Name)
			}
		})
	}
}
