package main

import "github.com/google/jsonschema-go/jsonschema"

func (a *application) diagramInputSchema(render bool) *jsonschema.Schema {
	schema := &jsonschema.Schema{
		Type: "object", AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		Properties: map[string]*jsonschema.Schema{
			"code":          {Type: "string", Description: "D2 source; provide exactly one of code or file_path"},
			"file_path":     {Type: "string", Description: "Relative .d2 path inside the server's explicit --input-dir"},
			"layout":        {Type: "string", Enum: []any{"dagre", "elk", "tala"}},
			"theme_id":      {Type: "integer", Description: "Built-in D2 theme ID (default 0)"},
			"dark_theme_id": {Type: "integer", Description: "Optional dark theme for SVG"},
			"sketch":        {Type: "boolean"},
			"padding":       {Type: "integer", Minimum: new(float64(0)), Maximum: new(float64(1000))},
			"scale":         {Type: "number", Minimum: new(0.1), Maximum: new(float64(4))},
			"center":        {Type: "boolean"},
			"board_path":    {Type: "array", Items: &jsonschema.Schema{Type: "string"}, MaxItems: new(16), Description: "Board-kind/name pairs, e.g. [layers, overview, scenarios, failure]; omitted selects root"},
		},
		OneOf: []*jsonschema.Schema{{Required: []string{"code"}}, {Required: []string{"file_path"}}},
	}
	if a.config.Store != nil {
		schema.Properties["document_id"] = &jsonschema.Schema{Type: "string", Description: "Stored complete source ID; requires revision; mutually exclusive with code and file_path"}
		schema.Properties["revision"] = &jsonschema.Schema{Type: "string", Description: "Exact expected document revision from store-d2"}
		schema.OneOf = append(schema.OneOf, &jsonschema.Schema{Required: []string{"document_id", "revision"}})
	}
	if render {
		schema.Properties["format"] = &jsonschema.Schema{Type: "string", Enum: []any{"png", "svg", "ascii"}}
		schema.Properties["ascii_mode"] = &jsonschema.Schema{Type: "string", Enum: []any{"extended", "standard"}}
		if a.output != nil {
			schema.Properties["output_name"] = &jsonschema.Schema{Type: "string", Description: "Filename inside --output-dir; .png, .svg, or .txt. Exclusive creation by default."}
			if a.config.AllowOverwrite {
				schema.Properties["overwrite"] = &jsonschema.Schema{Type: "boolean", Description: "Explicitly replace output_name; requires --allow-overwrite"}
			}
		}
	}
	return schema
}

func storeInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object", AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		Required: []string{"document_id", "expected_revision", "code"},
		Properties: map[string]*jsonschema.Schema{
			"document_id":       {Type: "string"},
			"expected_revision": {Type: "string"},
			"code":              {Type: "string"},
		},
	}
}
