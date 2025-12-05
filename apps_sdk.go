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

// appsResourceMeta returns the OpenAI Apps SDK metadata for resources
func appsResourceMeta() mcp.Meta {
	return mcp.Meta{
		"openai/outputTemplate":          appsResourceURI,
		"openai/widgetAccessible":        true,
		"openai/resultCanProduceWidget":  true,
		"openai/toolInvocation/invoking": "Rendering diagram",
		"openai/toolInvocation/invoked":  "Rendered diagram",
	}
}

// registerAppsResources registers the Apps SDK resources with the server
func registerAppsResources(s *mcp.Server) {
	html, err := loadAppsViewerTemplate()
	if err != nil {
		log.Fatalf("Failed to load Apps SDK viewer template: %v", err)
	}

	meta := appsResourceMeta()

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
