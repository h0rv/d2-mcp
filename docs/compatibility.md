# Inputs and limits

Rendering uses D2 defaults unless source settings or explicit arguments override
them. Dagre, ELK and TALA are bundled; PNG is rendered in Go.

## Source

- Send exactly one of complete `code` or a relative `.d2` `file_path` under
  `--input-dir`. Imports stay inside that root and are frozen before compilation.
- `board_path: ["layers", "overview"]` selects a board; omitted means root.
- Literal scalar variables, attribute-only classes/style rules, Markdown, code,
  and mathematical labels are supported. Expand structural templates, recursive
  substitutions and wildcard-created objects/edges before sending source.
- Source admission caps 256 KiB combined input, 32 files, 16 nesting/import
  levels, 10,000 estimated AST operations and 256 KiB expanded scalar data.
  These conservative estimates bound work before D2's non-cancellable compiler.

## Images and artifacts

- `--asset-dir` permits rooted local PNG/JPEG/GIF/SVG; `--allow-inline-assets`
  permits base64 images. Neither permits network fetching. SVG assets use D2's
  strict importer and are rasterized; unsupported features fail explicitly.
- Limits: 32 distinct assets, 1 MiB each, 8 MiB combined, 1024 pixels per side
  and 8 million combined pixels. GIF uses its first frame.
- Output: 16 MiB, 8192 pixels per side and 16 million pixels. ASCII has separate
  limits of 2000 cells per side and 1 million cells. Legends require SVG/PNG.
- PNG returns MCP image content, SVG an embedded text resource, ASCII text.
  Hash URIs identify bytes; they are not hosted download links.
- `--output-dir` enables file output. New names publish exclusively and require
  hard-link support. Replacement requires `--allow-overwrite` and an explicit
  `overwrite: true` with `output_name`; concurrent replacements are last writer
  wins. Cancellation removes only the request's private temporary file.

## Execution

Graphs are limited to 32 boards, 1000 objects and 2000 edges in total. Two
operations may run concurrently; excess calls return `busy`. The default timeout
is 30 seconds (`--timeout`, maximum 5 minutes), with cooperative cancellation.
Untrusted workloads still need process CPU/memory limits.

HTTP accepts one JSON-RPC object per POST, up to 2 MiB. Current-protocol
disconnects cancel work; older protocols retain the application deadline.
Legacy SSE is available but keeps a connection; prefer stateless HTTP.

## Upgrading from 1.2.0 to 2.0.0

The three tool names, default stdio transport and default PNG format remain.
[Install 2.0.0](installation.md), restart the client and refresh its discovered
tool schemas. Installing a skill alone does not upgrade the binary.

- **Rendering defaults change.** Without source settings or explicit options,
  2.0.0 uses Dagre, neutral theme 0, no sketch and padding 100. Version 1.2.0
  forced Dagre, GrapeSoda, sketch and padding 5. For those style settings, pass
  `layout: "dagre"`, `theme_id: 6`, `sketch: true`, `padding: 5`; D2 0.9.0 can
  still produce different geometry. PNG no longer needs `rsvg-convert`.
- **File access is opt-in.** Configure `--input-dir` and send relative `.d2`
  paths; arbitrary absolute paths are rejected. Imports stay within that root.
  Send exactly one of `code` or `file_path`, rather than both.
- **File output moves to an explicit root.** `--write-files` now also requires
  `--output-dir` and creates unique filenames there. It no longer writes beside
  the input. For a fixed filename use `output_name`; replacement requires both
  `--allow-overwrite` and `overwrite: true`.
- **Read result content by type.** SVG is an embedded text resource with
  `image/svg+xml`, replacing the old image-content encoding. PNG remains image
  content and ASCII remains text. Tool failures use `isError: true` with
  structured error codes and diagnostics; artifact metadata is also structured.
- **Inputs and HTTP requests are bounded.** The limits and supported D2 subset
  above apply even to source accepted by upstream D2. Image URLs are not fetched;
  local or inline images require explicit flags. HTTP accepts one JSON-RPC object
  per POST and rejects batches. Stdio and legacy SSE remain available.

The published 2.0.0 Linux binary also fixes the 1.2.0 diagnostic output that broke
strict MCP stdio clients. The optional viewer requires `--ui`; the shipped binary
still has no document-store backend. See the [release overview](releases/2.0.0.md).

The upstream Dagre ASCII arrowhead defect remains in D2 0.9.0. For affected
diagrams, try `layout: "elk"`; see the [reproduction and limits](ascii-rendering.md).
