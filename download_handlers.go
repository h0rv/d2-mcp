package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetDownloadURLHandler creates a temporary download URL for a diagram
func GetDownloadURLHandler(
	ctx context.Context,
	request *mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args, err := getArguments(request)
	if err != nil {
		return nil, err
	}

	// Get format (svg or png)
	format, ok := args["format"].(string)
	if !ok || format == "" {
		format = "svg" // default
	}

	// Get diagram data - either from data payload or by reading from arguments
	var data []byte
	var contentType string
	var filename string

	// Check if we have base64 data directly
	if dataB64, ok := args["data"].(string); ok && dataB64 != "" {
		data, err = base64.StdEncoding.DecodeString(dataB64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode data: %w", err)
		}
	} else if filePath, ok := args["file_path"].(string); ok && filePath != "" {
		// Read from file
		data, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
	} else {
		return nil, errors.New("either 'data' (base64) or 'file_path' must be provided")
	}

	// Set content type and filename based on format
	switch format {
	case "png":
		contentType = "image/png"
		filename = "diagram.png"
	case "svg":
		contentType = "image/svg+xml"
		filename = "diagram.svg"
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	// Store the download with 3 minute TTL (generous time for user to click)
	id := downloads.Store(data, contentType, filename, 180*time.Second)

	// Get the base URL for downloads
	baseURL := os.Getenv("DOWNLOAD_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080" // default
	}

	downloadURL := fmt.Sprintf("%s/download/%s", baseURL, id)

	log.Printf("[Download] Created temporary URL: %s (expires in 180s)", downloadURL)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("Download URL created (expires in 3 minutes): %s", downloadURL),
			},
		},
		// Return structured data for the widget to use
		StructuredContent: map[string]any{
			"downloadUrl": downloadURL,
			"expiresIn":   180,
			"format":      format,
		},
	}, nil
}
