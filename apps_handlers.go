package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"

	d2 "github.com/h0rv/d2-mcp/d2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func normalizeASCIIMode(mode string) (string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "", "extended", "unicode":
		return "extended", nil
	case "standard", "ascii":
		return "standard", nil
	default:
		return "", errors.New("invalid ASCII mode: " + mode)
	}
}

func RenderD2AppsHandler(
	ctx context.Context,
	request *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Printf("[Apps SDK] render_d2 tool called")

	args, err := getArguments(request)
	if err != nil {
		return nil, err
	}

	code, err := getCodeFromArgs(args)
	if err != nil {
		return nil, err
	}

	format := GlobalRenderFormat
	if formatArg, ok := args["format"].(string); ok && formatArg != "" {
		format = strings.ToLower(formatArg)
	}

	if _, ok := supportedFormatSet[format]; !ok {
		return nil, fmt.Errorf("unsupported format: %s (supported: %s)", format, strings.Join(supportedFormats, ", "))
	}

	payload := appsDiagramPayload{
		Code:   code,
		Format: format,
		Live:   true,
	}

	if filePath, ok := args["file_path"].(string); ok && filePath != "" {
		payload.SourceName = filePath
	}

	switch format {
	case "ascii":
		asciiMode, err := normalizeASCIIMode(GlobalASCIIMode)
		if err != nil {
			return nil, err
		}
		if modeArg, ok := args["ascii_mode"].(string); ok && modeArg != "" {
			asciiMode, err = normalizeASCIIMode(modeArg)
			if err != nil {
				return nil, err
			}
		}

		ascii, err := d2.RenderASCII(ctx, code, asciiMode)
		if err != nil {
			return nil, err
		}
		payload.ASCII = string(ascii)

	default:
		svg, err := d2.Render(ctx, code)
		if err != nil {
			return nil, err
		}

		payload.SVGBase64 = base64.StdEncoding.EncodeToString(svg)

		if format == "png" {
			png, err := SvgToPng(ctx, svg)
			if err != nil {
				return nil, err
			}
			payload.PNGBase64 = base64.StdEncoding.EncodeToString(png)
		}
	}

	// ChatGPT will fetch the HTML from the registered resource
	// We only return the data payload that the widget will read via window.openai.toolOutput
	summary := fmt.Sprintf("Rendered D2 diagram (%s format)", strings.ToUpper(format))
	log.Printf("[Apps SDK] render_d2 completed: format=%s, hasSVG=%v, hasPNG=%v, hasASCII=%v",
		format, payload.SVGBase64 != "", payload.PNGBase64 != "", payload.ASCII != "")

	return &mcp.CallToolResult{
		Meta: mcp.Meta{
			"openai/outputTemplate":          appsResourceURI,
			"openai/widgetAccessible":        true,
			"openai/resultCanProduceWidget":  true,
			"openai/toolInvocation/invoking": "Rendering diagram",
			"openai/toolInvocation/invoked":  "Rendered diagram",
		},
		Content: []mcp.Content{
			&mcp.TextContent{Text: summary},
		},
		StructuredContent: payload,
	}, nil
}
