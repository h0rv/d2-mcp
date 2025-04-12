package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func SvgToPng(ctx context.Context, svg []byte) ([]byte, error) {
	// Check if convert (ImageMagick) is available
	_, err := exec.LookPath("convert")
	if err != nil {
		return nil, fmt.Errorf("ImageMagick's convert command not found: %w", err)
	}

	// Create temporary files
	svgFile, err := os.CreateTemp("", "image-*.svg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp SVG file: %w", err)
	}
	defer os.Remove(svgFile.Name())

	pngFile, err := os.CreateTemp("", "image-*.png")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp PNG file: %w", err)
	}
	defer os.Remove(pngFile.Name())

	// Write SVG to temp file
	if _, err = svgFile.Write(svg); err != nil {
		return nil, fmt.Errorf("failed to write to temp SVG file: %w", err)
	}
	if err = svgFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp SVG file: %w", err)
	}

	// Convert SVG to PNG using ImageMagick
	cmd := exec.CommandContext(ctx, "convert", svgFile.Name(), pngFile.Name())
	if err = cmd.Run(); err != nil {
		return nil, fmt.Errorf("conversion failed: %w", err)
	}

	// Read the PNG file
	png, err := os.ReadFile(pngFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read PNG file: %w", err)
	}

	return png, nil
}
