package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var GlobalRenderFormat string
var GlobalWriteFiles bool
var GlobalASCIIMode string
var supportedFormats []string
var supportedFormatSet map[string]struct{}

func buildServerTools(formats []string) []server.ServerTool {
	formatList := strings.Join(formats, ", ")

	formatOptions := []mcp.ToolOption{
		mcp.WithDescription(fmt.Sprintf("Render a D2 diagram in %s format", formatList)),
		mcp.WithString("code", mcp.Description("The D2 code to render (either this or file_path is required)")),
		mcp.WithString("file_path", mcp.Description("Path to a D2 file to render (either this or code is required)")),
		mcp.WithString("format",
			mcp.Description(fmt.Sprintf("Optional output format override (%s)", formatList)),
			mcp.Enum(formats...),
		),
		mcp.WithString("ascii_mode",
			mcp.Description("ASCII rendering mode when format=ascii (extended, standard)"),
			mcp.Enum("extended", "standard"),
		),
	}

	return []server.ServerTool{
		{
			Tool: mcp.NewTool("compile-d2",
				mcp.WithDescription("Compile D2 code to validate and check for errors"),
				mcp.WithString("code", mcp.Description("The D2 code to compile (either this or file_path is required)")),
				mcp.WithString("file_path", mcp.Description("Path to a D2 file to compile (either this or code is required)")),
			),
			Handler: CompileD2Handler,
		},
		{
			Tool:    mcp.NewTool("render-d2", formatOptions...),
			Handler: RenderD2Handler,
		},
	}
}

func detectPNGSUpport() bool {
	if _, err := exec.LookPath("magick"); err == nil {
		return true
	}
	if _, err := exec.LookPath("convert"); err == nil {
		return true
	}
	return false
}

func main() {

	sseMode := flag.Bool("sse", false, "Enable SSE mode")
	port := flag.Int("port", 8080, "The port to run the server on")
	imageType := flag.String("image-type", "png", "The output format to render (png, svg, ascii)")
	writeFiles := flag.Bool("write-files", false, "Write output files to disk when using file_path (default: return base64)")
	asciiMode := flag.String("ascii-mode", "extended", "ASCII rendering mode when format is ascii (extended, standard)")
	flag.Parse()

	var (
		sseFlagSet  bool
		portFlagSet bool
	)
	flag.CommandLine.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "sse":
			sseFlagSet = true
		case "port":
			portFlagSet = true
		}
	})

	if !sseFlagSet {
		if env := os.Getenv("SSE_MODE"); strings.EqualFold(env, "true") {
			*sseMode = true
		}
	}

	if !portFlagSet {
		if env := os.Getenv("SSE_PORT"); env != "" {
			p, err := strconv.Atoi(env)
			if err != nil {
				log.Fatalf("Invalid SSE_PORT value: %s", env)
			}
			*port = p
		}
	}

	format := strings.ToLower(*imageType)
	if format != "png" && format != "svg" && format != "ascii" {
		log.Fatalf("Invalid render format: %s", *imageType)
	}

	mode := strings.ToLower(*asciiMode)
	if mode != "extended" && mode != "standard" {
		log.Fatalf("Invalid ASCII mode: %s", *asciiMode)
	}

	GlobalRenderFormat = format
	GlobalWriteFiles = *writeFiles
	GlobalASCIIMode = mode

	// Determine supported formats based on environment/tool availability.
	pngSupported := detectPNGSUpport()
	if !pngSupported {
		log.Println("Warning: PNG rendering disabled; install ImageMagick ('magick' or 'convert') to enable it.")
	}

	allFormats := []string{"png", "svg", "ascii"}
	for _, f := range allFormats {
		if f == "png" && !pngSupported {
			continue
		}
		supportedFormats = append(supportedFormats, f)
	}

	if len(supportedFormats) == 0 {
		log.Fatal("No rendering formats available; ensure at least SVG support is enabled")
	}

	supportedFormatSet = make(map[string]struct{}, len(supportedFormats))
	for _, f := range supportedFormats {
		supportedFormatSet[f] = struct{}{}
	}

	if _, ok := supportedFormatSet[GlobalRenderFormat]; !ok {
		fallback := supportedFormats[0]
		log.Printf("Warning: default format %s not available; falling back to %s", GlobalRenderFormat, fallback)
		GlobalRenderFormat = fallback
	}

	s := server.NewMCPServer(
		"d2-mcp",
		"1.0.0",
		server.WithLogging(),
	)

	s.SetTools(buildServerTools(supportedFormats)...)

	if *sseMode {
		url := fmt.Sprintf("http://localhost:%d", *port)
		sseServer := server.NewSSEServer(s, server.WithSSEEndpoint(url))
		log.Println("Starting d2-mcp service (mode: SSE) on " + url + "...")
		if err := sseServer.Start(fmt.Sprintf(":%d", *port)); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	} else {
		log.Println("Starting d2-mcp service (mode: stdio)...")
		if err := server.ServeStdio(s); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	}
}
