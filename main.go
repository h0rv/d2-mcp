package main

import (
	"flag"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var GlobalImageType string

var tools = []server.ServerTool{
	{
		Tool: mcp.NewTool("compile-d2",
			mcp.WithDescription("Compile D2 code to validate and check for errors"),
			mcp.WithString("code", mcp.Required(), mcp.Description("The D2 code to compile")),
		),
		Handler: CompileD2Handler,
	},
	{
		Tool: mcp.NewTool("render-d2",
			mcp.WithDescription("Render a D2 diagram into an image"),
			mcp.WithString("code", mcp.Required(), mcp.Description("The D2 code to render")),
		),
		Handler: RenderD2Handler,
	},
}

func main() {

	sseMode := flag.Bool("sse", false, "Enable SSE mode")
	imageType := flag.String("image-type", "png", "The type of image to render (png, svg)")
	flag.Parse()

	if *imageType != "png" && *imageType != "svg" {
		log.Fatalf("Invalid image type: %s", *imageType)
	}

	GlobalImageType = *imageType

	s := server.NewMCPServer(
		"d2-mcp",
		"1.0.0",
		server.WithLogging(),
	)

	s.SetTools(tools...)

	if *sseMode {
		url := "http://localhost:8080"
		sseServer := server.NewSSEServer(s, server.WithSSEEndpoint(url))
		log.Println("Starting d2-msp service (mode: SSE) on " + url + "...")
		if err := sseServer.Start(":8080"); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	} else {
		log.Println("Starting d2-mcp service (mode: stdio)...")
		if err := server.ServeStdio(s); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	}
}
