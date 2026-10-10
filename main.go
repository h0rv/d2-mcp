package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "1.3.0-dev"
const maxRequestBytes = 2 << 20

func (a *application) server() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "d2-mcp", Version: version}, &mcp.ServerOptions{Instructions: serverInstructions})
	properties := func(render bool) map[string]any {
		p := map[string]any{
			"code":          map[string]any{"type": "string", "description": "D2 source; provide exactly one of code or file_path"},
			"file_path":     map[string]any{"type": "string", "description": "Relative .d2 path inside the server's explicit --input-dir"},
			"layout":        map[string]any{"type": "string", "enum": []string{"dagre", "elk", "tala"}},
			"theme_id":      map[string]any{"type": "integer", "description": "Built-in D2 theme ID (default 0)"},
			"dark_theme_id": map[string]any{"type": "integer", "description": "Optional dark theme for SVG"},
			"sketch":        map[string]any{"type": "boolean"},
			"padding":       map[string]any{"type": "integer", "minimum": 0, "maximum": 1000},
			"scale":         map[string]any{"type": "number", "minimum": 0.1, "maximum": 4},
			"center":        map[string]any{"type": "boolean"},
			"board_path":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 16, "description": "Board-kind/name pairs, e.g. [layers, overview, scenarios, failure]; omitted selects root"},
		}
		if a.config.Store != nil {
			p["document_id"] = map[string]any{"type": "string", "description": "Stored complete source ID; requires revision; mutually exclusive with code and file_path"}
			p["revision"] = map[string]any{"type": "string", "description": "Exact expected document revision from store-d2"}
		}
		if render {
			p["format"] = map[string]any{"type": "string", "enum": []string{"png", "svg", "ascii"}}
			p["ascii_mode"] = map[string]any{"type": "string", "enum": []string{"extended", "standard"}}
			if a.output != nil {
				p["output_name"] = map[string]any{"type": "string", "description": "Filename inside --output-dir; .png, .svg, or .txt. Exclusive creation by default."}
				if a.config.AllowOverwrite {
					p["overwrite"] = map[string]any{"type": "boolean", "description": "Explicitly replace output_name; requires --allow-overwrite"}
				}
			}
		}
		return p
	}
	closed := false
	for _, render := range []bool{false, true} {
		destructive := render && a.config.AllowOverwrite
		name, description, handler := "compile-d2", "Validate D2 syntax, semantics, layout, and server limits", a.CompileD2Handler
		if render {
			name, description, handler = "render-d2", "Render one D2 diagram to PNG, SVG, or ASCII; returns an inline artifact unless file output is requested", a.RenderD2Handler
		}
		sourceChoices := []any{map[string]any{"required": []string{"code"}}, map[string]any{"required": []string{"file_path"}}}
		if a.config.Store != nil {
			sourceChoices = append(sourceChoices, map[string]any{"required": []string{"document_id", "revision"}})
		}
		s.AddTool(&mcp.Tool{Name: name, Description: description,
			InputSchema: map[string]any{"type": "object", "properties": properties(render), "additionalProperties": false, "oneOf": sourceChoices},
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: !render || a.output == nil, DestructiveHint: &destructive, OpenWorldHint: &closed, IdempotentHint: !render || a.output == nil},
		}, handler)
	}
	s.AddTool(&mcp.Tool{Name: "fetch_d2_cheat_sheet", Description: "Retrieve the bundled D2 quick reference and server limits", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed}}, FetchD2CheatSheetHandler)
	if a.config.Store != nil {
		a.addStoreTool(s)
	}
	return s
}

func httpHandler(s *mcp.Server, transport string) http.Handler {
	mux := http.NewServeMux()
	getServer := func(*http.Request) *mcp.Server { return s }
	if transport == "http" {
		mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: maxRequestBytes}))
	} else {
		legacy := mcp.NewSSEHandler(getServer, &mcp.SSEOptions{MaxRequestBodyBytes: maxRequestBytes})
		mux.Handle("/sse", legacy)
		mux.Handle("/message", legacy)
	}
	// MCP clients do not need cross-origin browser access. Check every method,
	// including legacy SSE GET, before handing the request to the SDK.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			if origin != scheme+"://"+r.Host {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

func run() error {
	var transport string
	flag.StringVar(&transport, "t", "stdio", "Transport (stdio, http, sse)")
	flag.StringVar(&transport, "transport", "stdio", "Transport (stdio, http, sse)")
	sse := flag.Bool("sse", false, "Legacy alias for --transport=sse")
	port := flag.Int("port", 8080, "HTTP/SSE port")
	host := flag.String("host", "127.0.0.1", "HTTP/SSE bind address; use 0.0.0.0 explicitly in containers")
	c := config{}
	flag.StringVar(&c.Format, "image-type", "png", "Default output format (png, svg, ascii)")
	flag.StringVar(&c.ASCIIMode, "ascii-mode", "extended", "ASCII connectors (extended, standard)")
	flag.StringVar(&c.InputDir, "input-dir", "", "Allow reading .d2 files/imports inside this existing directory")
	flag.StringVar(&c.OutputDir, "output-dir", "", "Allow creating artifacts inside this existing directory")
	flag.StringVar(&c.AssetDir, "asset-dir", "", "Allow bounded image/icon files relative to this existing directory; no network access")
	flag.BoolVar(&c.AllowInlineAssets, "allow-inline-assets", false, "Allow bounded base64 image/icon data URLs")
	flag.BoolVar(&c.AllowOverwrite, "allow-overwrite", false, "Allow explicit overwrite:true with output_name inside --output-dir")
	flag.BoolVar(&c.WriteFiles, "write-files", false, "Write file_path renders into --output-dir with unique names")
	flag.DurationVar(&c.Timeout, "timeout", 30*time.Second, "Per-call cooperative timeout, maximum 5m")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("d2-mcp " + version + " (D2 0.9.0)")
		return nil
	}
	var transportSet, portSet bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "t" || f.Name == "transport" {
			transportSet = true
		}
		if f.Name == "port" {
			portSet = true
		}
	})
	if !transportSet {
		if env := os.Getenv("MCP_TRANSPORT"); env != "" {
			transport = env
		}
		if strings.EqualFold(os.Getenv("SSE_MODE"), "true") {
			transport = "sse"
		}
	}
	if *sse {
		transport = "sse"
	}
	if !portSet {
		env := os.Getenv("PORT")
		if env == "" {
			env = os.Getenv("SSE_PORT")
		}
		if env != "" {
			p, err := strconv.Atoi(env)
			if err != nil {
				return errors.New("PORT/SSE_PORT must be an integer")
			}
			*port = p
		}
	}
	transport = strings.ToLower(strings.TrimSpace(transport))
	if transport != "stdio" && transport != "http" && transport != "sse" {
		return errors.New("transport must be stdio, http, or sse")
	}
	if *port < 1 || *port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	a, err := newApplication(c)
	if err != nil {
		return err
	}
	defer a.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s := a.server()
	if transport == "stdio" {
		return s.Run(ctx, &mcp.StdioTransport{MaxLineLength: maxRequestBytes})
	}
	if transport == "sse" {
		log.Print("SSE is retained for legacy clients; prefer --transport http")
	}
	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	httpServer := &http.Server{Addr: addr, Handler: httpHandler(s, transport), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	if transport == "http" {
		httpServer.WriteTimeout = c.Timeout + 5*time.Second
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	log.Printf("d2-mcp listening on %s (%s)", addr, transport)
	err = httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
