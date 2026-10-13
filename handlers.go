package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/h0rv/d2-mcp/d2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (a *application) execute(ctx context.Context, request *mcp.CallToolRequest, render bool) (*mcp.CallToolResult, error) {
	args, err := a.parseToolRequest(request.Params.Arguments, render)
	if err != nil {
		return errorResult("invalid_arguments", err), nil
	}
	ctx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return errorResult("cancelled", err), nil
	}
	select {
	case a.slots <- struct{}{}:
		defer func() { <-a.slots }()
	case <-ctx.Done():
		return errorResult("cancelled", ctx.Err()), nil
	default:
		return errorResult("busy", errors.New("renderer is busy; retry later")), nil
	}
	source, err := a.sourceWithDocument(ctx, args.source)
	if err != nil {
		return errorResult("invalid_arguments", err), nil
	}
	if a.assets != nil && args.source.documentID == "" {
		source.Assets = a.assets.FS()
	}
	source.InlineAssets = a.config.AllowInlineAssets
	prepared, err := d2.Prepare(ctx, source, args.options)
	if err != nil {
		return errorResult("invalid_d2", err), nil
	}
	if err := ctx.Err(); err != nil {
		return errorResult("cancelled", err), nil
	}
	settings := resolvedOptions(prepared, args.options.BoardPath)
	if !render {
		if err := ctx.Err(); err != nil {
			return errorResult("cancelled", err), nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "D2 script compiled successfully"}}, StructuredContent: compileOutput{Status: "ok", Valid: true, Options: settings}}, nil
	}
	data, err := d2.Render(ctx, prepared, args.format.name, args.asciiMode)
	if err != nil {
		return errorResult("render_failed", err), nil
	}
	if args.format.name == "svg" {
		if err := checkSVG(data); err != nil {
			return errorResult("unsupported_feature", err), nil
		}
	}
	if err := ctx.Err(); err != nil {
		return errorResult("cancelled", err), nil
	}
	mime := args.format.mimeType
	digest := sha256.Sum256(data)
	meta := artifact{MIMEType: mime, Bytes: len(data), SHA256: hex.EncodeToString(digest[:])}
	meta.URI = "urn:sha256:" + meta.SHA256
	var content []mcp.Content
	if args.outputName != "" || (a.config.WriteFiles && args.source.filePath != nil) {
		if err := ctx.Err(); err != nil {
			return errorResult("cancelled", err), nil
		}
		meta.Path, err = a.writeArtifact(ctx, args.outputName, args.format.name, data, args.overwrite)
		if err != nil {
			return errorResult("write_failed", err), nil
		}
		meta.URI = artifactFileURI(filepath.ToSlash(meta.Path))
		content = []mcp.Content{&mcp.TextContent{Text: "D2 diagram rendered to: " + meta.Path}, &mcp.ResourceLink{URI: meta.URI, Name: filepath.Base(meta.Path), MIMEType: mime, Size: ptr(int64(len(data)))}}
	} else {
		switch args.format.name {
		case "png":
			content = []mcp.Content{&mcp.ImageContent{Data: data, MIMEType: mime}}
		case "svg":
			content = []mcp.Content{&mcp.EmbeddedResource{Resource: &mcp.ResourceContents{URI: meta.URI, MIMEType: mime, Text: string(data)}}}
		case "ascii":
			content = []mcp.Content{&mcp.TextContent{Text: string(data)}}
		}
		if err := ctx.Err(); err != nil {
			return errorResult("cancelled", err), nil
		}
	}
	return &mcp.CallToolResult{Content: content, StructuredContent: renderOutput{Status: "ok", Artifact: meta, Options: settings}}, nil
}

func (a *application) CompileD2Handler(ctx context.Context, r *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return a.execute(ctx, r, false)
}

func (a *application) RenderD2Handler(ctx context.Context, r *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return a.execute(ctx, r, true)
}

func FetchD2CheatSheetHandler(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sheet, err := loadCheatSheet()
	if err != nil {
		return errorResult("internal_error", err), nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sheet}}}, nil
}

// D2 escapes XML; additionally reject active or external resource references
// in the emitted artifact, including links originating in Markdown labels.
func checkSVG(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	styleDepth := 0
	var stylesheet strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid SVG: %w", err)
		}
		switch node := token.(type) {
		case xml.ProcInst:
			if node.Target != "xml" {
				return errors.New("SVG processing instructions are disabled")
			}
		case xml.CharData:
			if styleDepth > 0 {
				stylesheet.Write(node)
			}
		case xml.EndElement:
			if node.Name.Local == "style" {
				styleDepth--
				if err := checkCSS(stylesheet.String(), true); err != nil {
					return err
				}
				stylesheet.Reset()
			}
		}
		element, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if element.Name.Local == "script" || element.Name.Local == "foreignObject" || element.Name.Local == "set" || strings.HasPrefix(element.Name.Local, "animate") {
			return errors.New("active SVG content is disabled")
		}
		if element.Name.Local == "style" {
			styleDepth++
		}
		for _, attr := range element.Attr {
			if strings.HasPrefix(strings.ToLower(attr.Name.Local), "on") {
				return errors.New("SVG event handlers are disabled")
			}
			if attr.Name.Local == "href" && !strings.HasPrefix(attr.Value, "#") && !(element.Name.Local == "a" && d2.SafeLink(attr.Value)) && !(element.Name.Local == "image" && safeEmbeddedPNG(attr.Value)) {
				return errors.New("external SVG resources and unsafe links are disabled")
			}
			switch strings.ToLower(attr.Name.Local) {
			case "style", "fill", "stroke", "filter", "mask", "clip-path", "cursor", "marker", "marker-start", "marker-mid", "marker-end":
				if err := checkCSS(attr.Value, false); err != nil {
					return err
				}
			}
		}
	}
}

// Input is an absolute native path after filepath.ToSlash. Keep Windows drive
// letters in the URI path, and UNC server names in the authority.
func artifactFileURI(path string) string {
	if strings.HasPrefix(path, "//?/UNC/") {
		path = "//" + strings.TrimPrefix(path, "//?/UNC/")
	} else if strings.HasPrefix(path, "//?/") {
		path = strings.TrimPrefix(path, "//?/")
	}
	if strings.HasPrefix(path, "//") {
		host, rest, _ := strings.Cut(strings.TrimPrefix(path, "//"), "/")
		return (&url.URL{Scheme: "file", Host: host, Path: "/" + rest}).String()
	}
	if len(path) >= 3 && path[1] == ':' && path[2] == '/' {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func ptr[T any](v T) *T { return &v }

// D2's built-in grain pattern embeds a PNG. Allow bounded raster data, never SVG
// data (which can itself contain links/scripts). Enabled source assets have
// already been decoded/rasterized and normalized to PNG by the asset loader.
func safeEmbeddedPNG(value string) bool {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(value, prefix) || len(value) > base64.StdEncoding.EncodedLen(d2.MaxAssetBytes)+len(prefix) {
		return false
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return false
	}
	size, err := png.DecodeConfig(bytes.NewReader(data))
	return err == nil && size.Width > 0 && size.Height > 0 && size.Width <= 1024 && size.Height <= 1024
}
