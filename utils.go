package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

type svgToPNGConverter struct {
	name string
	path string
	args func(inputPath, outputPath string) []string
}

func findSVGToPNGConverter() (*svgToPNGConverter, error) {
	return findSVGToPNGConverterWithLookPath(exec.LookPath)
}

func findSVGToPNGConverterWithLookPath(lookPath func(string) (string, error)) (*svgToPNGConverter, error) {
	path, err := lookPath("rsvg-convert")
	if err != nil {
		return nil, fmt.Errorf("PNG conversion requires librsvg's 'rsvg-convert' on PATH")
	}

	return &svgToPNGConverter{
		name: "rsvg-convert",
		path: path,
		args: func(inputPath, outputPath string) []string {
			return []string{"-f", "png", "-o", outputPath, inputPath}
		},
	}, nil
}

func SvgToPng(ctx context.Context, svg []byte) ([]byte, error) {
	converter, err := findSVGToPNGConverter()
	if err != nil {
		return nil, err
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
	if err = pngFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp PNG file: %w", err)
	}

	// Use librsvg because ImageMagick does not reliably render D2's embedded
	// WOFF fonts, which can turn labels into tofu squares in PNG output.
	cmd := exec.CommandContext(ctx, converter.path, converter.args(svgFile.Name(), pngFile.Name())...)

	// Capture both stdout and stderr for better error reporting
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s conversion failed: %w\noutput: %s", converter.name, err, string(output))
	}

	// Read the PNG file
	png, err := os.ReadFile(pngFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read PNG file: %w", err)
	}

	return png, nil
}
