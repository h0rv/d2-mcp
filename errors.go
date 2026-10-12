package main

import (
	"context"
	"errors"

	"github.com/d2lang/d2/d2parser"
	"github.com/h0rv/d2-mcp/d2"
	"github.com/h0rv/d2-mcp/document"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type diagnostic struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}
type toolError struct {
	Code        string       `json:"code"`
	Message     string       `json:"message"`
	Diagnostics []diagnostic `json:"diagnostics,omitempty"`
}

func errorResult(code string, err error) *mcp.CallToolResult {
	var d2err *d2.Error
	if errors.As(err, &d2err) {
		code = d2err.Code
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		code = "cancelled"
	}
	detail := toolError{Code: code, Message: err.Error()}
	var parse *d2parser.ParseError
	if errors.As(err, &parse) {
		detail.Code = "invalid_d2"
		for _, e := range parse.Errors {
			detail.Diagnostics = append(detail.Diagnostics, diagnostic{e.Range.Path, e.Range.Start.Line + 1, e.Range.Start.Column + 1, e.Message})
		}
	}
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: detail.Message}}, StructuredContent: map[string]any{"status": "error", "error": detail}}
}

func documentError(err error) error {
	code := "store_failed"
	message := "document store failed"
	if errors.Is(err, document.ErrNotFound) {
		code, message = "document_not_found", err.Error()
	}
	if errors.Is(err, document.ErrConflict) {
		code, message = "revision_conflict", err.Error()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &d2.Error{Code: code, Message: message}
}
