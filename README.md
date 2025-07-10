# d2-mcp

A Model Context Protocol (MCP) server for working with [D2: Declarative Diagramming](https://d2lang.com/), enabling seamless integration of diagram creation and validation into your development workflow.

**Tools:**

* Compile D2 Code
    * Validate D2 syntax and catch errors before rendering
    * Get immediate feedback on diagram structure and syntax
    * Accepts either direct code or file path to D2 file
* Render Diagrams
    * Generate diagrams for visual feedback and refinement
    * Support for both vector (SVG) and raster (PNG) output formats
    * Accepts either direct code or file path to D2 file

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

# Run in stdio mode (default - for MCP clients)
docker run --rm -i d2-mcp

# Run in stdio mode with filesystem access
docker run --rm -i -v $(pwd):/data d2-mcp

# Run in SSE mode (HTTP server)
docker run --rm SSE_MODE=true -p 8080:8080 -e d2-mcp

# Run in SSE mode with filesystem access
docker run --rm -e SSE_MODE=true -p 8080:8080 -v $(pwd):/data d2-mcp
```

### Option 5: Run Container Image

```bash
# Run in stdio mode (default - for MCP clients)
docker run --rm -i ghcr.io/h0rv/d2-mcp:main

# Run in stdio mode with filesystem access
docker run --rm -i -v $(pwd):/data ghcr.io/h0rv/d2-mcp:main

# Run in SSE mode (HTTP server)
docker run --rm -e SSE_MODE=true -p 8080:8080 ghcr.io/h0rv/d2-mcp:main

# Run in SSE mode with filesystem access
docker run --rm -e SSE_MODE=true -p 8080:8080 -v $(pwd):/data ghcr.io/h0rv/d2-mcp:main
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

**Using Binary:**
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

**Using Binary with file output:**
```json
{
    "mcpServers": {
        "d2": {
            "command": "/YOUR/ABSOLUTE/PATH/d2-mcp",
            "args": ["--image-type", "png", "--write-files"]
        }
    }
}
```

**Using Docker:**
```json
{
    "mcpServers": {
        "d2": {
            "command": "docker",
            "args": ["run", "--rm", "-i", "d2-mcp"]
        }
    }
}
```

**Using Docker with filesystem access:**
```json
{
    "mcpServers": {
        "d2": {
            "command": "docker",
            "args": ["run", "--rm", "-i", "-v", "./:/data", "d2-mcp", "--image-type", "png"]
        }
    }
}
```

## Usage

The MCP server supports two ways to provide D2 code:

1. **Direct code parameter:** Pass D2 code directly as a string (good for simple diagrams)
2. **File path parameter:** Pass a path to a D2 file (recommended for complex diagrams or to avoid JSON serialization issues)

### Input/Output Behavior

**By default (without `--write-files` flag):**
- **compile-d2**: Validates D2 code/file and returns success/error message
- **render-d2**: Renders the diagram and **returns base64 encoded image** for display

**With `--write-files` flag enabled:**
- **compile-d2**: Same as default behavior
- **render-d2**: When using `file_path`, **writes output to a file** (e.g., `diagram.d2` → `diagram.png`)

**Example usage:**
```bash
# Default behavior: always returns base64 image
echo '{"code": "A -> B: hello"}' | d2-mcp render-d2
echo '{"file_path": "diagram.d2"}' | d2-mcp render-d2

# With --write-files flag: writes to file when using file_path
echo '{"file_path": "diagram.d2"}' | d2-mcp --write-files render-d2
```

## Development

### Debugging

```bash
npx @modelcontextprotocol/inspector /YOUR/ABSOLUTE/PATH/d2-mcp/d2-mcp
```
