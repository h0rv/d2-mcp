package main

import (
	"embed"
	"fmt"
)

//go:embed d2/CHEATSHEET.md
var docFS embed.FS

const serverInstructions = `Validate and render complete D2 diagrams with D2 0.9.0.

- compile-d2 validates syntax, semantics, layout, and server limits.
- render-d2 returns PNG image content, an embedded SVG resource, or ASCII text.
- fetch_d2_cheat_sheet returns supported syntax and usage examples.
- Supply exactly one of code or a relative file_path inside the configured input directory.
- Select dagre, elk, or tala with layout. Set theme_id, sketch, padding, scale, and center per call.
- Explicit options override source vars.d2-config; otherwise use D2 defaults.
- File access is opt-in. Output writes require an output directory; overwrite needs explicit server and call opt-ins.
- Select a board with board_path pairs, e.g. ["layers", "overview"]. Omitted means root; each call returns one board.
- Image assets need --asset-dir or --allow-inline-assets; network image fetching is disabled. Use native shapes by default.
- Use literal scalar variables, attribute-only classes/style rules, and explicit objects/connections. Structural templates and block-string interpolation are unsupported.
- Source admission bounds combined nesting/import depth to 16, projected AST work to 10000, and expanded source/scalars to 256 KiB. Math labels are limited to 4096 bytes.
- Errors are tool results with isError, readable text, and structured diagnostics when available.
- HTTP is stateless: resend complete source/options. There is no default document storage.
- Send one JSON-RPC object per HTTP POST; batches are rejected on all protocol versions.
- If a deployment configures a document-store adapter, explicitly retain document_id/revision
  from store-d2 and pass both on later calls. Result metadata is not automatically round-tripped.
`

func loadCheatSheet() (string, error) {
	data, err := docFS.ReadFile("d2/CHEATSHEET.md")
	if err != nil {
		return "", fmt.Errorf("failed to load D2 cheat sheet: %w", err)
	}
	return string(data), nil
}
