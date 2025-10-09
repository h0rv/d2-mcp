package main

import (
	"embed"
	"fmt"
)

//go:embed d2/CHEATSHEET.md
var docFS embed.FS

const serverInstructions = `Use the d2-mcp server to validate and render D2 diagrams directly from your MCP client.

Available tools:

- compile-d2 — Validates D2 source code or a .d2 file path. Always run this before rendering large diagrams so you can surface syntax issues quickly.
- render-d2 — Renders diagrams to PNG, SVG, or ASCII. You can override the format per call with the "format" argument and optionally set "ascii_mode" for ASCII output.
- fetch_d2_cheat_sheet — Returns a Markdown quick reference with common shapes, styling tips, and example snippets for D2.

Usage tips:

- You may provide either the "code" argument (raw D2) or "file_path". When both are absent the tools return a friendly error that can be shown to the user.
- The server only advertises PNG support when ImageMagick ("magick" or "convert") is available on PATH.
- ASCII output is ideal for LLM consumption; choose "standard" mode for pure ASCII connectors or "extended" for Unicode box drawing characters.
- The server supports stdio, SSE, and streamable HTTP transports. Adjust with --transport (stdio|sse|http) or MCP_TRANSPORT environment variables.
`

func loadCheatSheet() (string, error) {
	data, err := docFS.ReadFile("d2/CHEATSHEET.md")
	if err != nil {
		return "", fmt.Errorf("failed to load D2 cheat sheet: %w", err)
	}
	return string(data), nil
}
