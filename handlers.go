package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	d2 "github.com/h0rv/d2-mcp/d2"

	"github.com/mark3labs/mcp-go/mcp"
)

// getArguments safely returns request arguments as a map, decoding JSON when needed.
func getArguments(request mcp.CallToolRequest) map[string]any {
	if args := request.GetArguments(); args != nil {
		return args
	}

	var decoded map[string]any
	if err := request.BindArguments(&decoded); err == nil && decoded != nil {
		return decoded
	}

	return map[string]any{}
}

// getCodeFromRequest extracts D2 code from either the "code" parameter or by reading from "file_path"
func getCodeFromArgs(args map[string]any) (string, error) {
	// Check if code is provided directly
	if code, ok := args["code"].(string); ok && code != "" {
		return code, nil
	}

	// Check if file_path is provided
	if filePath, ok := args["file_path"].(string); ok && filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", errors.New("failed to read file: " + err.Error())
		}
		return string(content), nil
	}

	return "", errors.New("either 'code' or 'file_path' parameter must be provided")
}

// generateOutputFilename creates an output filename based on input filename and requested format
func generateOutputFilename(inputPath, format string) string {
	dir := filepath.Dir(inputPath)
	base := filepath.Base(inputPath)

	// Remove .d2 extension if present
	if strings.HasSuffix(base, ".d2") {
		base = strings.TrimSuffix(base, ".d2")
	}

	// Add appropriate extension
	var ext string
	switch format {
	case "png":
		ext = ".png"
	case "svg":
		ext = ".svg"
	case "ascii":
		ext = ".txt"
	default:
		ext = "." + format
	}

	return filepath.Join(dir, base+ext)
}

func CompileD2Handler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := getArguments(request)
	code, err := getCodeFromArgs(args)
	if err != nil {
		return nil, err
	}

	_, _, compileErr, otherErr := d2.Compile(ctx, code)
	if otherErr != nil {
		return nil, otherErr
	}

	if compileErr != nil {
		return mcp.NewToolResultError(compileErr.Error()), nil
	}

	return mcp.NewToolResultText("D2 script compiled successfully"), nil
}

func RenderD2Handler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := getArguments(request)
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

	if format == "ascii" {
		normalize := func(mode string) (string, error) {
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

		asciiMode, err := normalize(GlobalASCIIMode)
		if err != nil {
			return nil, err
		}

		if modeArg, ok := args["ascii_mode"].(string); ok && modeArg != "" {
			asciiMode, err = normalize(modeArg)
			if err != nil {
				return nil, err
			}
		}

		ascii, err := d2.RenderASCII(ctx, code, asciiMode)
		if err != nil {
			return nil, err
		}

		if GlobalWriteFiles {
			if filePath, ok := args["file_path"].(string); ok && filePath != "" {
				outputPath := generateOutputFilename(filePath, format)
				if err := os.WriteFile(outputPath, ascii, 0644); err != nil {
					return nil, errors.New("failed to write output file: " + err.Error())
				}
				return mcp.NewToolResultText("D2 diagram rendered to: " + outputPath), nil
			}
		}

		return mcp.NewToolResultText(string(ascii)), nil
	}

	svg, err := d2.Render(ctx, code)
	if err != nil {
		return nil, err
	}

	var (
		img     []byte
		imgType string
	)

	if format == "png" {
		png, err := SvgToPng(ctx, svg)
		if err != nil {
			return nil, err
		}
		img = png
		imgType = "image/png"
	} else {
		img = svg
		imgType = "image/svg+xml"
	}

	// Write to file if --write-files flag is enabled AND file_path was provided
	if GlobalWriteFiles {
		if filePath, ok := args["file_path"].(string); ok && filePath != "" {
			outputPath := generateOutputFilename(filePath, format)
			if err := os.WriteFile(outputPath, img, 0644); err != nil {
				return nil, errors.New("failed to write output file: " + err.Error())
			}
			return mcp.NewToolResultText("D2 diagram rendered to: " + outputPath), nil
		}
	}

	// Always return base64 encoded image by default
	imageEncoded := base64.StdEncoding.EncodeToString(img)
	return mcp.NewToolResultImage("D2 diagram", imageEncoded, imgType), nil
}

func FetchD2CheatSheetHandler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	cheatSheet, err := loadCheatSheet()
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(cheatSheet), nil
}
