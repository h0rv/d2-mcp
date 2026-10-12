package main

// These wire types cover the schema vocabulary used by our tools. In particular,
// false additionalProperties and empty properties must both survive encoding.
type toolInputSchema struct {
	Type                 string                    `json:"type"`
	Properties           map[string]schemaProperty `json:"properties"`
	AdditionalProperties bool                      `json:"additionalProperties"`
	Required             []string                  `json:"required,omitempty"`
	OneOf                []schemaChoice            `json:"oneOf,omitempty"`
}

type schemaProperty struct {
	Type        string          `json:"type"`
	Description string          `json:"description,omitempty"`
	Enum        []string        `json:"enum,omitempty"`
	Minimum     *float64        `json:"minimum,omitempty"`
	Maximum     *float64        `json:"maximum,omitempty"`
	Items       *schemaProperty `json:"items,omitempty"`
	MaxItems    int             `json:"maxItems,omitempty"`
}

type schemaChoice struct {
	Required []string `json:"required"`
}

func (a *application) diagramInputSchema(render bool) toolInputSchema {
	schema := toolInputSchema{
		Type: "object",
		Properties: map[string]schemaProperty{
			"code":          {Type: "string", Description: "D2 source; provide exactly one of code or file_path"},
			"file_path":     {Type: "string", Description: "Relative .d2 path inside the server's explicit --input-dir"},
			"layout":        {Type: "string", Enum: []string{"dagre", "elk", "tala"}},
			"theme_id":      {Type: "integer", Description: "Built-in D2 theme ID (default 0)"},
			"dark_theme_id": {Type: "integer", Description: "Optional dark theme for SVG"},
			"sketch":        {Type: "boolean"},
			"padding":       {Type: "integer", Minimum: new(float64(0)), Maximum: new(float64(1000))},
			"scale":         {Type: "number", Minimum: new(0.1), Maximum: new(float64(4))},
			"center":        {Type: "boolean"},
			"board_path":    {Type: "array", Items: &schemaProperty{Type: "string"}, MaxItems: 16, Description: "Board-kind/name pairs, e.g. [layers, overview, scenarios, failure]; omitted selects root"},
		},
		OneOf: []schemaChoice{{Required: []string{"code"}}, {Required: []string{"file_path"}}},
	}
	if a.config.Store != nil {
		schema.Properties["document_id"] = schemaProperty{Type: "string", Description: "Stored complete source ID; requires revision; mutually exclusive with code and file_path"}
		schema.Properties["revision"] = schemaProperty{Type: "string", Description: "Exact expected document revision from store-d2"}
		schema.OneOf = append(schema.OneOf, schemaChoice{Required: []string{"document_id", "revision"}})
	}
	if render {
		schema.Properties["format"] = schemaProperty{Type: "string", Enum: []string{"png", "svg", "ascii"}}
		schema.Properties["ascii_mode"] = schemaProperty{Type: "string", Enum: []string{"extended", "standard"}}
		if a.output != nil {
			schema.Properties["output_name"] = schemaProperty{Type: "string", Description: "Filename inside --output-dir; .png, .svg, or .txt. Exclusive creation by default."}
			if a.config.AllowOverwrite {
				schema.Properties["overwrite"] = schemaProperty{Type: "boolean", Description: "Explicitly replace output_name; requires --allow-overwrite"}
			}
		}
	}
	return schema
}

func storeInputSchema() toolInputSchema {
	return toolInputSchema{
		Type:     "object",
		Required: []string{"document_id", "expected_revision", "code"},
		Properties: map[string]schemaProperty{
			"document_id":       {Type: "string"},
			"expected_revision": {Type: "string"},
			"code":              {Type: "string"},
		},
	}
}
