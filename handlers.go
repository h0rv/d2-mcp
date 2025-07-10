package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"

	d2 "github.com/h0rv/d2-mcp/d2"

	"github.com/mark3labs/mcp-go/mcp"
)

// getCodeFromRequest extracts D2 code from either the "code" parameter or by reading from "file_path"
func getCodeFromRequest(request mcp.CallToolRequest) (string, error) {
	// Check if code is provided directly
	if code, ok := request.Params.Arguments["code"].(string); ok && code != "" {
		return code, nil
	}

	// Check if file_path is provided
	if filePath, ok := request.Params.Arguments["file_path"].(string); ok && filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", errors.New("failed to read file: " + err.Error())
		}
		return string(content), nil
	}

	return "", errors.New("either 'code' or 'file_path' parameter must be provided")
}

// generateOutputFilename creates an output filename based on input filename and image type
func generateOutputFilename(inputPath, imageType string) string {
	dir := filepath.Dir(inputPath)
	base := filepath.Base(inputPath)

	// Remove .d2 extension if present
	if strings.HasSuffix(base, ".d2") {
		base = strings.TrimSuffix(base, ".d2")
	}

	// Add appropriate extension
	var ext string
	if imageType == "png" {
		ext = ".png"
	} else {
		ext = ".svg"
	}

	return filepath.Join(dir, base+ext)
}

func CompileD2Handler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	code, err := getCodeFromRequest(request)
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
	code, err := getCodeFromRequest(request)
	if err != nil {
		return nil, err
	}

	svg, err := d2.Render(ctx, code)
	if err != nil {
		return nil, err
	}

	var (
		img     []byte
		imgType string
	)

	if GlobalImageType == "png" {
		png, err := SvgToPng(ctx, svg)
		if err != nil {
			return nil, err
		}
		img = png
		imgType = "image/png"
	} else if GlobalImageType == "svg" {
		img = svg
		imgType = "image/svg+xml"
	} else {
		return nil, errors.New("invalid image type: " + GlobalImageType)
	}

	// Write to file if --write-files flag is enabled AND file_path was provided
	if GlobalWriteFiles {
		if filePath, ok := request.Params.Arguments["file_path"].(string); ok && filePath != "" {
			outputPath := generateOutputFilename(filePath, GlobalImageType)
			err := os.WriteFile(outputPath, img, 0644)
			if err != nil {
				return nil, errors.New("failed to write output file: " + err.Error())
			}
			return mcp.NewToolResultText("D2 diagram rendered to: " + outputPath), nil
		}
	}

	// Always return base64 encoded image by default
	imageEncoded := base64.StdEncoding.EncodeToString(img)
	return mcp.NewToolResultImage("D2 diagram", imageEncoded, imgType), nil
}
