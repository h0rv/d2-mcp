# d2-mcp

A Model Context Protocol (MCP) server for working with [D2: Declarative Diagramming](https://d2lang.com/), enabling seamless integration of diagram creation and validation into your development workflow.

**Tools:**

* Compile D2 Code
    * Validate D2 syntax and catch errors before rendering
    * Get immediate feedback on diagram structure and syntax
    * Accepts either direct code or file path to D2 file
* Render Diagrams
    * Generate diagrams for visual feedback and refinement
    * Support PNG, SVG, and ASCII output formats
    * Accepts either direct code or file path to D2 file
* Fetch D2 Cheat Sheet
    * Returns a Markdown reference covering shapes, styling, and transport usage

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
            "args": ["run", "--rm", "-i", "ghcr.io/h0rv/d2-mcp:main", "--image-type", "svg"]
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
            "args": [
                "run", "--rm", "-i",
                "-v", "./:/data",
                "ghcr.io/h0rv/d2-mcp:main",
                "--image-type", "ascii",
                "--ascii-mode", "standard",
                "--write-files"
            ]
        }
    }
}
```

## Rendering Formats

The server returns PNG output by default. Override globally when starting the binary:

```bash
./d2-mcp --image-type svg        # SVG output
./d2-mcp --image-type ascii      # ASCII output with Unicode box drawing characters
./d2-mcp --image-type ascii --ascii-mode standard  # ASCII output restricted to basic ASCII chars
```

Inside MCP tool calls, pass the optional `format` argument (`png`, `svg`, `ascii`) and, when `ascii`, the `ascii_mode` argument (`extended`, `standard`) to switch formats per request.

### Docker Usage Examples

Run the container with default PNG output over stdio:

```bash
docker run --rm -i ghcr.io/h0rv/d2-mcp:main
```

Switch to Unicode ASCII diagrams and capture responses as plain text:

```bash
docker run --rm -i ghcr.io/h0rv/d2-mcp:main --image-type ascii
```

Use basic ASCII characters and write rendered files back into your working tree (requires a bind mount):

```bash
docker run --rm -i \
  -v "$(pwd)":/data \
  ghcr.io/h0rv/d2-mcp:main \
  --image-type ascii \
  --ascii-mode standard \
  --write-files
```

Expose the SSE server on port 8080 while emitting SVG:

```bash
docker run --rm -e SSE_MODE=true -p 8080:8080 ghcr.io/h0rv/d2-mcp:main --image-type svg
```

Expose the streamable HTTP transport (default endpoint `/mcp`) for use with MCP clients that expect the new protocol:

```bash
docker run --rm -p 8080:8080 ghcr.io/h0rv/d2-mcp:main --transport http --image-type svg
```

### Cheat Sheet Tool

Retrieve the built-in quick reference as Markdown:

```json
{
  "tool": "fetch_d2_cheat_sheet"
}
```

The cheat sheet highlights common shapes, layout tips, and ASCII-friendly patterns, making it ideal support material for downstream LLM prompts.

## Transports

The server defaults to stdio transport for CLI-driven MCP clients. Switch transports per run:

- `--transport stdio`: default for local CLI integrations.
- `--transport sse`: legacy Server-Sent Events transport (alias: `--sse`).
- `--transport http`: streamable HTTP transport; combine with `-p`/`--port` when running in Docker or containers.

Environment overrides:

- `MCP_TRANSPORT` sets the transport (`stdio`, `sse`, `http`) when flags are not provided.
- `PORT` (or the legacy `SSE_PORT`) sets the listening port for SSE/HTTP transports.
- `SSE_MODE=true` retains backwards compatibility by selecting the SSE transport.

## Tool Reference

| Tool | Description | Key Arguments |
| ---- | ----------- | ------------- |
| `compile-d2` | Validates D2 source and surfaces syntax errors. | `code` (string) or `file_path` (string) |
| `render-d2` | Renders diagrams to PNG, SVG, or ASCII (ASCII is LLM-friendly). | `code`/`file_path`, `format` (`png`, `svg`, `ascii`), `ascii_mode` (`extended`, `standard`) |
| `fetch_d2_cheat_sheet` | Returns a Markdown cheat sheet with examples and best practices. | _None_ |

Tip: Run `compile-d2` first to validate, then call `render-d2` with the same payload for the final output.

## Development

### Debugging

```bash
npx @modelcontextprotocol/inspector /YOUR/ABSOLUTE/PATH/d2-mcp/d2-mcp
```
