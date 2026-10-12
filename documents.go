package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/h0rv/d2-mcp/d2"
	"github.com/h0rv/d2-mcp/document"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func validReference(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.:", r)) {
			return false
		}
	}
	return true
}

// Stored documents are portable complete sources: no server-local imports.
type noImports struct{}

func (noImports) Open(string) (fs.File, error) {
	return nil, errors.New("stored documents must be self-contained; imports are disabled")
}
func (a *application) sourceWithDocument(ctx context.Context, args sourceInput) (d2.Source, error) {
	if err := ctx.Err(); err != nil {
		return d2.Source{}, err
	}
	if args.documentID == "" && args.revision == "" {
		return a.source(args)
	}
	if a.config.Store == nil {
		return d2.Source{}, errors.New("document storage is not configured")
	}
	if args.code != nil || args.filePath != nil || !validReference(args.documentID) || !validReference(args.revision) {
		return d2.Source{}, errors.New("provide document_id and revision without code or file_path")
	}
	doc, err := a.config.Store.Get(ctx, args.documentID)
	if err != nil {
		return d2.Source{}, documentError(err)
	}
	if err := ctx.Err(); err != nil {
		return d2.Source{}, err
	}
	if doc.Revision != args.revision {
		return d2.Source{}, documentError(document.ErrConflict)
	}
	if doc.ID != args.documentID || len(doc.Code) > d2.MaxSourceBytes || strings.TrimSpace(doc.Code) == "" {
		return d2.Source{}, documentError(errors.New("invalid snapshot"))
	}
	return d2.Source{Code: doc.Code, Path: "input.d2", FS: noImports{}}, nil
}
func (a *application) addStoreTool(s *mcp.Server) {
	closed, destructive := false, true
	s.AddTool(&mcp.Tool{Name: "store-d2", Description: "Opt-in: store a complete self-contained D2 source using atomic revision checks. Empty expected_revision creates only; retain the returned ID and revision explicitly.",
		InputSchema: storeInputSchema(), Annotations: &mcp.ToolAnnotations{OpenWorldHint: &closed, DestructiveHint: &destructive},
	}, a.StoreD2Handler)
}
func (a *application) StoreD2Handler(ctx context.Context, r *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		ID       string  `json:"document_id"`
		Expected *string `json:"expected_revision"`
		Code     string  `json:"code"`
	}
	decoder := json.NewDecoder(bytes.NewReader(r.Params.Arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return errorResult("invalid_arguments", err), nil
	}
	if a.config.Store == nil {
		return errorResult("invalid_arguments", errors.New("document storage is not configured")), nil
	}
	if !validReference(args.ID) || args.Expected == nil || (*args.Expected != "" && !validReference(*args.Expected)) || strings.TrimSpace(args.Code) == "" {
		return errorResult("invalid_arguments", errors.New("provide code, a valid document_id, and expected_revision (empty only to create)")), nil
	}
	ctx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return errorResult("cancelled", err), nil
	}
	select {
	case a.slots <- struct{}{}:
		defer func() { <-a.slots }()
	default:
		return errorResult("busy", errors.New("renderer is busy; retry later")), nil
	}
	if _, err := d2.Prepare(ctx, d2.Source{Code: args.Code, Path: "input.d2", FS: noImports{}, InlineAssets: a.config.AllowInlineAssets}, d2.Options{}); err != nil {
		return errorResult("invalid_d2", err), nil
	}
	if err := ctx.Err(); err != nil {
		return errorResult("cancelled", err), nil
	}
	doc, err := a.config.Store.Put(ctx, args.ID, *args.Expected, args.Code)
	if err != nil {
		return errorResult("store_failed", documentError(err)), nil
	}
	if doc.ID != args.ID || !validReference(doc.Revision) || doc.Revision == *args.Expected || doc.Code != args.Code {
		return errorResult("store_failed", errors.New("store returned an invalid committed snapshot; reconcile before retrying")), nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Stored document %s at revision %s", doc.ID, doc.Revision)}}, StructuredContent: storeOutput{Status: "ok", DocumentID: doc.ID, Revision: doc.Revision}}, nil
}
