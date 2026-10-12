package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"maps"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	diagramAppURI      = "ui://d2/viewer"
	diagramAppMIME     = "text/html;profile=mcp-app"
	diagramUIExtension = "io.modelcontextprotocol/ui"
)

// The release binary serves a self-contained, reviewed build; no Node runtime,
// CDN, UI listener, or client-controlled resource path is involved.
//
//go:embed apps/dist/viewer.html
var diagramAppHTML string

func diagramAppMeta() mcp.Meta {
	return mcp.Meta{"ui": map[string]any{
		"csp": map[string]any{
			"connectDomains":  []string{},
			"resourceDomains": []string{},
			"frameDomains":    []string{},
			"baseUriDomains":  []string{},
		},
	}}
}

func addDiagramApp(s *mcp.Server) {
	s.AddResource(&mcp.Resource{
		URI: diagramAppURI, Name: "d2-viewer", Title: "D2 diagram viewer",
		Description: "Optional MCP Apps viewer for inline D2 artifacts",
		MIMEType:    diagramAppMIME, Meta: diagramAppMeta(),
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: diagramAppURI, MIMEType: diagramAppMIME, Text: diagramAppHTML, Meta: diagramAppMeta(),
		}}}, nil
	})
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, req)
			if err != nil || method != "tools/list" {
				return result, err
			}
			request, ok := req.(*mcp.ListToolsRequest)
			if !ok || !offerDiagramApp(request) {
				return result, nil
			}
			list, ok := result.(*mcp.ListToolsResult)
			if !ok {
				return result, nil
			}
			// The SDK's list contains shared registered tool pointers. Copy every
			// object we modify so mixed clients cannot alter each other's metadata.
			copyList := *list
			copyList.Tools = slices.Clone(list.Tools)
			for i, tool := range copyList.Tools {
				if tool.Name == "render-d2" {
					copyTool := *tool
					copyTool.Meta = maps.Clone(tool.Meta)
					if copyTool.Meta == nil {
						copyTool.Meta = mcp.Meta{}
					}
					copyTool.Meta["ui"] = map[string]any{"resourceUri": diagramAppURI}
					copyList.Tools[i] = &copyTool
				}
			}
			return &copyList, nil
		}
	})
}

func offerDiagramApp(req *mcp.ListToolsRequest) bool {
	caps := req.ClientCapabilities()
	if caps == nil {
		if req.Params != nil {
			if _, explicit := req.Params.Meta[mcp.MetaKeyClientCapabilities]; explicit {
				// An explicit null or malformed capability value is not evidence
				// of legacy transport information loss.
				return false
			}
		}
		// Pre-2026-07-28 stateless HTTP has no retained initialization state.
		// With --ui explicitly enabled, advertise the optional metadata for
		// those requests; older hosts can ignore it and still use the artifact.
		// Known capability sets, current HTTP, stdio, and SSE negotiate below.
		return req.Session != nil && req.Session.ID() == "" &&
			req.Extra != nil && req.Extra.Header != nil &&
			req.ProtocolVersion() != "" && req.ProtocolVersion() < "2026-07-28"
	}
	var settings struct {
		MIMETypes []string `json:"mimeTypes"`
	}
	encoded, err := json.Marshal(caps.Extensions[diagramUIExtension])
	return err == nil && json.Unmarshal(encoded, &settings) == nil && slices.Contains(settings.MIMETypes, diagramAppMIME)
}
