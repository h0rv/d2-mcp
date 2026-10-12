package main

import "github.com/h0rv/d2-mcp/d2"

type artifact struct {
	MIMEType string `json:"mime_type"`
	Bytes    int    `json:"bytes"`
	SHA256   string `json:"sha256"`
	URI      string `json:"uri"`
	Path     string `json:"path,omitempty"`
}

// effectiveOptions reports the values resolved by D2 after source configuration
// and explicit overrides have been applied. BoardPath is always a JSON array.
type effectiveOptions struct {
	Layout      string   `json:"layout"`
	ThemeID     int64    `json:"theme_id"`
	DarkThemeID *int64   `json:"dark_theme_id,omitempty"`
	Sketch      bool     `json:"sketch"`
	Padding     int64    `json:"padding"`
	BoardPath   []string `json:"board_path"`
	Center      bool     `json:"center"`
	Scale       float64  `json:"scale"`
}

func resolvedOptions(p *d2.Prepared, boardPath []string) effectiveOptions {
	options := effectiveOptions{
		Layout: p.Layout, ThemeID: *p.Options.ThemeID, DarkThemeID: p.Options.DarkThemeID,
		Sketch: *p.Options.Sketch, Padding: *p.Options.Pad, Center: *p.Options.Center,
		BoardPath: append([]string{}, boardPath...), Scale: 1,
	}
	if p.Options.Scale != nil {
		options.Scale = *p.Options.Scale
	}
	return options
}

type compileOutput struct {
	Status  string           `json:"status"`
	Valid   bool             `json:"valid"`
	Options effectiveOptions `json:"options"`
}

type renderOutput struct {
	Status   string           `json:"status"`
	Artifact artifact         `json:"artifact"`
	Options  effectiveOptions `json:"options"`
}

type storeOutput struct {
	Status     string `json:"status"`
	DocumentID string `json:"document_id"`
	Revision   string `json:"revision"`
}
