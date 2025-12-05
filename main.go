package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var GlobalRenderFormat string
var GlobalWriteFiles bool
var GlobalASCIIMode string
var supportedFormats []string
var supportedFormatSet map[string]struct{}
var GlobalAppsSDKEnabled bool

func containsFormat(formats []string, target string) bool {
	for _, f := range formats {
		if f == target {
			return true
		}
	}
	return false
}

// boolPtr returns a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}

func registerServerTools(s *mcp.Server, formats []string) {
	formatList := strings.Join(formats, ", ")

	// compile-d2 tool
	s.AddTool(&mcp.Tool{
		Name:        "compile-d2",
		Description: "Compile D2 code to validate and check for errors",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "The D2 code to compile (either this or file_path is required)",
				},
				"file_path": map[string]any{
					"type":        "string",
					"description": "Path to a D2 file to compile (either this or code is required)",
				},
			},
		},
	}, CompileD2Handler)

	// render_d2 tool - unified for both standard and Apps SDK modes
	renderSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "The D2 code to render (either this or file_path is required)",
			},
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to a D2 file to render (either this or code is required)",
			},
			"format": map[string]any{
				"type":        "string",
				"description": fmt.Sprintf("Optional output format override (%s)", formatList),
				"enum":        formats,
			},
		},
	}

	if containsFormat(formats, "ascii") {
		props := renderSchema["properties"].(map[string]any)
		props["ascii_mode"] = map[string]any{
			"type":        "string",
			"description": "ASCII rendering mode when format=ascii (extended, standard)",
			"enum":        []string{"extended", "standard"},
		}
	}

	// Build the tool with conditional metadata
	tool := &mcp.Tool{
		Name:        "render_d2",
		Description: "Renders a D2 diagram with interactive preview",
		InputSchema: renderSchema,
	}

	// Add Apps SDK metadata if enabled
	if GlobalAppsSDKEnabled {
		tool.Meta = mcp.Meta{
			"openai/outputTemplate":          appsResourceURI,
			"openai/widgetAccessible":        true,
			"openai/resultCanProduceWidget":  true,
			"openai/toolInvocation/invoking": "Rendering diagram",
			"openai/toolInvocation/invoked":  "Rendered diagram",
		}
		tool.Title = "Render D2 Diagram"
		tool.Annotations = &mcp.ToolAnnotations{
			Title:           "Render D2 Diagram",
			ReadOnlyHint:    true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		}
	}

	s.AddTool(tool, RenderD2Handler)

	// fetch_d2_cheat_sheet tool
	s.AddTool(&mcp.Tool{
		Name:        "fetch_d2_cheat_sheet",
		Description: "Retrieve the bundled D2 quick-reference cheat sheet in Markdown.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, FetchD2CheatSheetHandler)
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
	flag.StringVar(&transport, "t", "stdio", "Transport type (stdio, http)")
	flag.StringVar(&transport, "transport", "stdio", "Transport type (stdio, http)")
	sseFlag := flag.Bool("sse", false, "Enable SSE transport (deprecated, use --transport=http)")
	port := flag.Int("port", 8080, "The port to run the server on")
	imageType := flag.String("image-type", "png", "The output format to render (png, svg, ascii)")
	writeFiles := flag.Bool("write-files", false, "Write output files to disk when using file_path (default: return base64)")
	asciiMode := flag.String("ascii-mode", "extended", "ASCII rendering mode when format is ascii (extended, standard)")
	enableAppsSDK := flag.Bool("enable-apps-sdk", false, "Register ChatGPT Apps SDK resources and tools")
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
		log.Println("Warning: --sse is deprecated, use --transport=http instead.")
		transport = "http"
	}

	if !transportFlagSet && !*sseFlag {
		if env := os.Getenv("MCP_TRANSPORT"); env != "" {
			transport = env
		}
	}

	if !transportFlagSet && !*sseFlag {
		if env := os.Getenv("SSE_MODE"); strings.EqualFold(env, "true") {
			transport = "http"
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

	// Map old "sse" to "http"
	if transport == "sse" {
		transport = "http"
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
	GlobalAppsSDKEnabled = *enableAppsSDK

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

	// Create the MCP server
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "d2-mcp",
		Version: "1.0.0",
	}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})

	// Register standard tools
	registerServerTools(s, supportedFormats)

	// Register Apps SDK resources if enabled
	if *enableAppsSDK {
		registerAppsResources(s)
	}

	switch transport {
	case "stdio":
		log.Println("Starting d2-mcp service (transport: stdio)...")
		if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	case "http":
		addr := fmt.Sprintf(":%d", *port)
		mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			return s
		}, nil)
		// Wrap with logging and path handling
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[HTTP] %s %s", r.Method, r.URL.Path)
			// Strip /mcp prefix if present
			if r.URL.Path == "/mcp" || r.URL.Path == "/mcp/" {
				r.URL.Path = "/"
			} else if len(r.URL.Path) > 4 && r.URL.Path[:5] == "/mcp/" {
				r.URL.Path = r.URL.Path[4:]
			}
			mcpHandler.ServeHTTP(w, r)
		})
		log.Printf("Starting d2-mcp service (transport: http) on http://localhost%s/mcp ...", addr)
		if err := http.ListenAndServe(addr, handler); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	default:
		log.Fatalf("Invalid transport type: %s. Must be 'stdio' or 'http'", transport)
	}
}
