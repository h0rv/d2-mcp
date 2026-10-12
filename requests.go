package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/h0rv/d2-mcp/d2"
)

// Keep the wire type separate: null, omitted and explicit false/zero have
// established meanings, and encoding/json reports this type in its diagnostics.
type toolArgs struct {
	Code       *string `json:"code,omitempty"`
	DocumentID string  `json:"document_id,omitempty"`
	Revision   string  `json:"revision,omitempty"`
	FilePath   *string `json:"file_path,omitempty"`
	Format     string  `json:"format,omitempty"`
	ASCIIMode  string  `json:"ascii_mode,omitempty"`
	OutputName string  `json:"output_name,omitempty"`
	Overwrite  bool    `json:"overwrite,omitempty"`

	d2.Options
}

// Source selection and I/O happen after admission to an execution slot. Keeping
// them there preserves cancellation/busy error priority and filesystem bounds.
type sourceInput struct {
	code, filePath       *string
	documentID, revision string
}

type diagramRequest struct {
	source     sourceInput
	options    d2.Options
	format     outputFormat
	asciiMode  string // canonical "extended" or "standard"
	outputName string
	overwrite  bool
}

// A parsed format carries its MIME type, so handlers cannot construct a
// mismatched artifact or look up an unchecked string in a MIME dictionary.
type outputFormat struct {
	name, mimeType string
}

func parseOutputFormat(name string) (outputFormat, error) {
	switch name {
	case "png":
		return outputFormat{name: "png", mimeType: "image/png"}, nil
	case "svg":
		return outputFormat{name: "svg", mimeType: "image/svg+xml"}, nil
	case "ascii":
		return outputFormat{name: "ascii", mimeType: "text/plain"}, nil
	default:
		return outputFormat{}, errors.New("format must be png, svg, or ascii")
	}
}

func parseASCIIMode(mode string) (string, error) {
	switch mode {
	case "standard", "ascii":
		return "standard", nil
	case "extended", "unicode":
		return "extended", nil
	default:
		return "", errors.New("ascii_mode must be extended or standard")
	}
}

func (a *application) parseToolRequest(data json.RawMessage, render bool) (diagramRequest, error) {
	var args toolArgs
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return diagramRequest{}, err
	}
	if !render && (args.Format != "" || args.ASCIIMode != "" || args.OutputName != "" || args.Overwrite) {
		return diagramRequest{}, errors.New("format, ascii_mode, output_name, and overwrite apply only to render-d2")
	}
	formatName := a.config.Format
	if args.Format != "" {
		formatName = strings.ToLower(args.Format)
	}
	format, err := parseOutputFormat(formatName)
	if err != nil {
		return diagramRequest{}, err
	}
	modeName := a.config.ASCIIMode
	if args.ASCIIMode != "" {
		modeName = strings.ToLower(args.ASCIIMode)
	}
	mode, err := parseASCIIMode(modeName)
	if err != nil {
		return diagramRequest{}, err
	}
	if args.OutputName != "" && a.output == nil {
		return diagramRequest{}, &d2.Error{Code: "file_access_denied", Message: "output_name requires --output-dir"}
	}
	if args.Overwrite && (!a.config.AllowOverwrite || args.OutputName == "") {
		return diagramRequest{}, &d2.Error{Code: "file_access_denied", Message: "overwrite requires --allow-overwrite and an explicit output_name"}
	}
	return diagramRequest{
		source:  sourceInput{code: args.Code, filePath: args.FilePath, documentID: args.DocumentID, revision: args.Revision},
		options: args.Options, format: format, asciiMode: mode,
		outputName: args.OutputName, overwrite: args.Overwrite,
	}, nil
}
