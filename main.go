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

func containsFormat(formats []string, target string) bool {
	for _, f := range formats {
		if f == target {
			return true
		}
	}
	return false
}

func buildServerTools(formats []string) []server.ServerTool {
	formatList := strings.Join(formats, ", ")

	renderOptions := []mcp.ToolOption{
		mcp.WithDescription(fmt.Sprintf("Render a D2 diagram in %s format", formatList)),
		mcp.WithString("code", mcp.Description("The D2 code to render (either this or file_path is required)")),
		mcp.WithString("file_path", mcp.Description("Path to a D2 file to render (either this or code is required)")),
		mcp.WithString("format",
			mcp.Description(fmt.Sprintf("Optional output format override (%s)", formatList)),
			mcp.Enum(formats...),
		),
	}

	if containsFormat(formats, "ascii") {
		renderOptions = append(renderOptions, mcp.WithString("ascii_mode",
			mcp.Description("ASCII rendering mode when format=ascii (extended, standard)"),
			mcp.Enum("extended", "standard"),
		))
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
			Tool:    mcp.NewTool("render-d2", renderOptions...),
			Handler: RenderD2Handler,
		},
	}
}

func detectPNGSupport() bool {
	if _, err := exec.LookPath("magick"); err == nil {
		return true
	}
	if _, err := exec.LookPath("convert"); err == nil {
		return true
	}
	return false
}

func main() {

	var transport string
	flag.StringVar(&transport, "t", "stdio", "Transport type (stdio, sse, http)")
	flag.StringVar(&transport, "transport", "stdio", "Transport type (stdio, sse, http)")
	sseFlag := flag.Bool("sse", false, "Enable SSE transport (deprecated, use --transport=sse)")
	port := flag.Int("port", 8080, "The port to run the server on")
	imageType := flag.String("image-type", "png", "The output format to render (png, svg, ascii)")
	writeFiles := flag.Bool("write-files", false, "Write output files to disk when using file_path (default: return base64)")
	asciiMode := flag.String("ascii-mode", "extended", "ASCII rendering mode when format is ascii (extended, standard)")
	flag.Parse()

	var (
		transportFlagSet bool
		portFlagSet      bool
	)
	flag.CommandLine.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "t", "transport":
			transportFlagSet = true
		case "port":
			portFlagSet = true
		}
	})

	if *sseFlag {
		log.Println("Warning: --sse is deprecated, use --transport=sse instead.")
		transport = "sse"
	}

	if !transportFlagSet && !*sseFlag {
		if env := os.Getenv("MCP_TRANSPORT"); env != "" {
			transport = env
		}
	}

	if !transportFlagSet && !*sseFlag {
		if env := os.Getenv("SSE_MODE"); strings.EqualFold(env, "true") {
			transport = "sse"
		}
	}

	if !portFlagSet {
		if env := os.Getenv("PORT"); env != "" {
			p, err := strconv.Atoi(env)
			if err != nil {
				log.Fatalf("Invalid PORT value: %s", env)
			}
			*port = p
		} else if env := os.Getenv("SSE_PORT"); env != "" {
			p, err := strconv.Atoi(env)
			if err != nil {
				log.Fatalf("Invalid SSE_PORT value: %s", env)
			}
			*port = p
		}
	}

	transport = strings.ToLower(strings.TrimSpace(transport))
	if transport == "" {
		transport = "stdio"
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
	pngSupported := detectPNGSupport()
	if !pngSupported {
		log.Println("Warning: PNG rendering disabled; install ImageMagick ('magick' or 'convert') to enable it.")
	}

	allFormats := []string{"png", "svg", "ascii"}
	supportedFormats = make([]string, 0, len(allFormats))
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

	switch transport {
	case "stdio":
		log.Println("Starting d2-mcp service (transport: stdio)...")
		if err := server.ServeStdio(s); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	case "sse":
		url := fmt.Sprintf("http://localhost:%d", *port)
		sseServer := server.NewSSEServer(s, server.WithSSEEndpoint(url))
		log.Println("Starting d2-mcp service (transport: sse) on " + url + "...")
		if err := sseServer.Start(fmt.Sprintf(":%d", *port)); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	case "http":
		addr := fmt.Sprintf(":%d", *port)
		httpServer := server.NewStreamableHTTPServer(s)
		log.Println("Starting d2-mcp service (transport: http) on http://localhost" + addr + "/mcp ...")
		if err := httpServer.Start(addr); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	default:
		log.Fatalf("Invalid transport type: %s. Must be 'stdio', 'sse', or 'http'", transport)
	}
}
