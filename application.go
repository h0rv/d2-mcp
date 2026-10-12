package main

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/h0rv/d2-mcp/document"
)

// config belongs to this Go server; run populates it from CLI flags. Rendering
// calls the D2 Go APIs directly and never invokes a D2 command or shell.
type config struct {
	Format, ASCIIMode, InputDir, OutputDir, AssetDir string
	WriteFiles                                       bool
	UI                                               bool
	AllowOverwrite, AllowInlineAssets                bool
	Timeout                                          time.Duration
	Store                                            document.Store
}
type application struct {
	config                config
	input, output, assets *os.Root
	outputPath            string
	slots                 chan struct{}
}

func newApplication(c config) (*application, error) {
	if c.Format != "png" && c.Format != "svg" && c.Format != "ascii" {
		return nil, errors.New("image-type must be png, svg, or ascii")
	}
	if c.ASCIIMode != "extended" && c.ASCIIMode != "standard" {
		return nil, errors.New("ascii-mode must be extended or standard")
	}
	if c.Timeout <= 0 || c.Timeout > 5*time.Minute {
		return nil, errors.New("timeout must be positive and at most 5m")
	}
	if c.WriteFiles && c.OutputDir == "" {
		return nil, errors.New("--write-files requires --output-dir")
	}
	if c.AllowOverwrite && c.OutputDir == "" {
		return nil, errors.New("--allow-overwrite requires --output-dir")
	}
	a := &application{config: c, slots: make(chan struct{}, 2)}
	var err error
	if c.InputDir != "" {
		a.input, err = os.OpenRoot(c.InputDir)
		if err != nil {
			return nil, err
		}
	}
	if c.OutputDir != "" {
		a.outputPath, err = filepath.Abs(c.OutputDir)
		if err == nil {
			a.output, err = os.OpenRoot(a.outputPath)
		}
		if err != nil {
			a.Close()
			return nil, err
		}
	}
	if c.AssetDir != "" {
		a.assets, err = os.OpenRoot(c.AssetDir)
		if err != nil {
			a.Close()
			return nil, err
		}
	}
	return a, nil
}
func (a *application) Close() {
	if a.input != nil {
		a.input.Close()
	}
	if a.output != nil {
		a.output.Close()
	}
	if a.assets != nil {
		a.assets.Close()
	}
}
