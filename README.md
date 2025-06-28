# d2-mcp

A Model Context Protocol (MCP) server for working with [D2: Declarative Diagramming](https://d2lang.com/), enabling seamless integration of diagram creation and validation into your development workflow.

**Tools:**

* Compile D2 Code
    * Validate D2 syntax and catch errors before rendering
    * Get immediate feedback on diagram structure and syntax
* Render Diagrams
    * Generate diagrams for visual feedback and refinement
    * Support for both vector (SVG) and raster (PNG) output formats

## Install

### Option 1: Install Binary Release

```bash
```

### Option 2: Install via `go`

```bash
go install github.com/h0rv/d2-mcp@latest
```

### Option 3: Build Locally

```bash
git clone https://github.com/h0rv/d2-mcp.git
cd d2-mcp
go build .
```

### Option 4: Build Image Locally

```bash
docker build . -t d2-mcp
docker run --rm -p 8080:8080 d2-mcp
```

### Option 5: Run Container Image

```bash
docker run --rm -p 8080:8080 ghcr.io/h0rv/d2-mcp:main
```

## Setup with MCP Client

MacOS:

```bash
# Claude Desktop
$EDITOR ~/Library/Application\ Support/Claude/claude_desktop_config.json
# OTerm:
$EDITOR ~/Library/Application\ Support/oterm/config.json
```

Add the `d2` MCP server to your respective MCP Clients config:

```json
{
    "mcpServers": {
        "d2": {
            "command": "/YOUR/ABSOLUTE/PATH/d2-mcp",
            "args": ["--image-type", "png"]
        }
    }
}
```

## Development

### Debugging

```bash
npx @modelcontextprotocol/inspector /YOUR/ABSOLUTE/PATH/d2-mcp/d2-mcp
```
