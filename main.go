package main

import (
	"context"
	"errors"
	"flag"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	d2 "github.com/h0rv/d2-mcp/d2"
)

func main() {

	sseMode := flag.Bool("sse", false, "Enable SSE mode")
	flag.Parse()

	s := server.NewMCPServer(
		"d2-mcp",
		"1.0.0",
	)

	registerD2Tools(s)

	if *sseMode {
		url := "http://localhost:8080"
		sseServer := server.NewSSEServer(s, server.WithSSEEndpoint(url))
		log.Println("Starting SSE server on " + url)
		if err := sseServer.Start(":8080"); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	} else {
		log.Println("Starting stdio server...")
		if err := server.ServeStdio(s); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	}
}

func registerD2Tools(s *server.MCPServer) {
	renderTool := mcp.NewTool("render-d2",
		mcp.WithDescription("Render a D2 diagram into a SVG"),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("The D2 script to render"),
		),
	)
	s.AddTool(renderTool, renderD2Handler)

	compileTool := mcp.NewTool("compile-d2",
		mcp.WithDescription("Compile a D2 script into a D2 graph"),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("The D2 script to compile"),
		),
	)
	s.AddTool(compileTool, compileD2Handler)
}

func renderD2Handler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	code, ok := request.Params.Arguments["code"].(string)
	if !ok {
		return nil, errors.New("code must be a string")
	}

	svg, err := d2.Render(ctx, code)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(string(svg)), nil
}

func compileD2Handler(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	code, ok := request.Params.Arguments["code"].(string)
	if !ok {
		return nil, errors.New("code must be a string")
	}

	_, _, err := d2.Compile(ctx, code)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(""), nil
}
