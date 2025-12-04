package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"strings"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
)

const (
	appsResourceURI  = "ui://d2_viewer"
	appsResourceMIME = "text/html+skybridge"
)

// appsDiagramPayload carries the data the ChatGPT Apps iframe renders.
type appsDiagramPayload struct {
	Code       string `json:"code"`
	Format     string `json:"format"`
	SourceName string `json:"sourceName,omitempty"`
	SVGBase64  string `json:"svgBase64,omitempty"`
	PNGBase64  string `json:"pngBase64,omitempty"`
	ASCII      string `json:"ascii,omitempty"`
	Live       bool   `json:"live,omitempty"`
}

//go:embed apps/viewer.html
var appsFS embed.FS

func generateThemeOptions() string {
	// Combine light and dark catalogs
	allThemes := append(d2themescatalog.LightCatalog, d2themescatalog.DarkCatalog...)

	var options strings.Builder
	for _, theme := range allThemes {
		// Format theme name for display
		name := formatThemeName(theme.Name)
		options.WriteString(fmt.Sprintf(`<option value="%d">%s</option>`, theme.ID, name))
		options.WriteString("\n          ")
	}

	return options.String()
}

func formatThemeName(name string) string {
	// Convert "NeutralDefault" to "Neutral default"
	var result strings.Builder
	for i, r := range name {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune(' ')
		}
		if i == 0 {
			result.WriteRune(r)
		} else {
			result.WriteRune(unicode.ToLower(r))
		}
	}
	return result.String()
}

func loadAppsViewerTemplate() (string, error) {
	data, err := appsFS.ReadFile("apps/viewer.html")
	if err != nil {
		return "", fmt.Errorf("failed to load Apps SDK viewer template: %w", err)
	}

	html := string(data)

	// Replace {{THEME_OPTIONS}} placeholder with dynamically generated theme list
	html = strings.Replace(html, "{{THEME_OPTIONS}}", generateThemeOptions(), 1)

	return html, nil
}

// appsToolMeta returns the OpenAI Apps SDK metadata for tools
func appsToolMeta() mcp.Meta {
	return mcp.Meta{
		"openai/outputTemplate":          appsResourceURI,
		"openai/widgetAccessible":        true,
		"openai/resultCanProduceWidget":  true,
		"openai/toolInvocation/invoking": "Rendering diagram",
		"openai/toolInvocation/invoked":  "Rendered diagram",
	}
}

// registerAppsTools registers the Apps SDK tools with the server
func registerAppsTools(s *mcp.Server, formats []string) {
	formatList := strings.Join(formats, ", ")

	// Build input schema
	inputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "The D2 code to render (either this or file_path is required)",
			},
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to a D2 file to render (either this or code is required)",
			},
			"format": map[string]any{
				"type":        "string",
				"description": fmt.Sprintf("Optional output format override (%s)", formatList),
				"enum":        formats,
			},
			"ascii_mode": map[string]any{
				"type":        "string",
				"description": "ASCII rendering mode when format=ascii (extended, standard)",
				"enum":        []string{"extended", "standard"},
			},
		},
	}

	// Create the tool with Apps SDK metadata embedded in Meta field
	tool := &mcp.Tool{
		Meta:        appsToolMeta(),
		Name:        "render_d2",
		Title:       "Render D2 Diagram",
		Description: "Renders a D2 diagram with interactive preview",
		InputSchema: inputSchema,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Render D2 Diagram",
			ReadOnlyHint:    true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
	}

	s.AddTool(tool, RenderD2AppsHandler)
}

// registerAppsResources registers the Apps SDK resources with the server
func registerAppsResources(s *mcp.Server) {
	html, err := loadAppsViewerTemplate()
	if err != nil {
		log.Fatalf("Failed to load Apps SDK viewer template: %v", err)
	}

	meta := appsToolMeta()

	// Register the resource
	s.AddResource(&mcp.Resource{
		Meta:        meta,
		URI:         appsResourceURI,
		Name:        "D2 Diagram Viewer",
		Description: "Interactive D2 diagram viewer widget",
		MIMEType:    appsResourceMIME,
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		log.Printf("[Apps SDK] Resource requested: %s", req.Params.URI)
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      appsResourceURI,
					MIMEType: appsResourceMIME,
					Text:     html,
					Meta:     meta,
				},
			},
		}, nil
	})

	// Also register as a resource template
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Meta:        meta,
		URITemplate: appsResourceURI,
		Name:        "D2 Diagram Viewer",
		Description: "Interactive D2 diagram viewer widget",
		MIMEType:    appsResourceMIME,
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		log.Printf("[Apps SDK] Resource template requested: %s", req.Params.URI)
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      appsResourceURI,
					MIMEType: appsResourceMIME,
					Text:     html,
					Meta:     meta,
				},
			},
		}, nil
	})
}

// boolPtr returns a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}
