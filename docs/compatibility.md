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

## Upgrading from 1.2

Tool names, stdio and PNG defaults remain. Files now require explicit roots,
SVG is a text resource, and HTTP batches are rejected. The renderer uses D2
defaults and native PNG rather than the previous converter. Check client tool
schemas before sending new options; installing a skill does not upgrade a binary.
