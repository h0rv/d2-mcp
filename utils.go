package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func SvgToPng(ctx context.Context, svg []byte) ([]byte, error) {
	// Check which ImageMagick command is available
	// ImageMagick v7 uses "magick", v6 uses "convert"
	var magickCmd string
	if _, err := exec.LookPath("magick"); err == nil {
		magickCmd = "magick"
	} else if _, err := exec.LookPath("convert"); err == nil {
		magickCmd = "convert"
	} else {
		return nil, fmt.Errorf("ImageMagick not found: neither 'magick' nor 'convert' command available")
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
	cmd := exec.CommandContext(ctx, magickCmd, svgFile.Name(), pngFile.Name())

	// Capture both stdout and stderr for better error reporting
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("conversion failed: %w\nImageMagick output: %s", err, string(output))
	}

	// Read the PNG file
	png, err := os.ReadFile(pngFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read PNG file: %w", err)
	}

	return png, nil
}
