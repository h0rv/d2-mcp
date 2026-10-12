# d2-mcp

A small Go MCP server for validating and rendering [D2](https://d2lang.com/) diagrams. Send complete source and rendering options; receive PNG, SVG, or text. The default server keeps no documents, artifacts, or HTTP sessions. It needs no D2 executable, browser, or PNG converter.

This branch uses **D2 0.9.0**, **Go 1.27.2**, and the official **Go MCP SDK 1.8.0**. D2's Dagre, ELK, and open-source TALA layouts are bundled. See [inputs and limits](docs/compatibility.md).

## Install

Follow [install commands](docs/installation.md), then [connect Claude, Codex, Pi, or VS Code](docs/agent-installation.md). The refreshed renderer is not released yet; the guide distinguishes building this branch from installing the older published binary.

## Clients and transports

Stdio remains the default. For Claude Desktop or another client using `mcpServers`:

```json
{
  "mcpServers": {
    "d2": {
      "command": "/absolute/path/d2-mcp",
      "args": ["--image-type", "png"]
    }
  }
}
```

For a stateless Streamable HTTP endpoint:

```sh
./d2-mcp --transport http --host 127.0.0.1 --port 8080
```

Connect a Streamable HTTP MCP client to `http://127.0.0.1:8080/mcp`. For example, a VS Code `.vscode/mcp.json` entry:

```json
{
  "servers": {
    "d2": {"type": "http", "url": "http://127.0.0.1:8080/mcp"}
  }
}
```

HTTP uses JSON responses, no session ID, and no background event stream. Every call carries its source/options; subsequent calls may go to another replica. GET/DELETE on `/mcp` return 405. Current MCP `2026-07-28` discovery and legacy initialization are handled by the SDK; integration tests cover `2025-03-26`, `2025-06-18`, `2025-11-25`, and `2026-07-28` over HTTP. Stdio and legacy SSE retain their supported initialization protocols.

Send exactly one JSON-RPC object per HTTP POST. Batches are rejected on every protocol version, including legacy SSE message posts, to prevent unbounded aggregation of tool results. Disconnects cancel current-protocol (`2026-07-28`) work cooperatively; the SDK retains older protocol calls until their application deadline.

Legacy SSE remains available at `/sse` with `--transport sse` (or `--sse`). Its SDK-managed connections are stateful and need a stable server connection. Prefer HTTP for new integrations. `MCP_TRANSPORT`, `PORT`, and legacy `SSE_MODE`/`SSE_PORT` are still accepted; explicit transport/port flags take precedence, and `--sse` selects SSE.

HTTP binds to loopback by default, rejects cross-origin browser requests, and retains the SDK's localhost/DNS-rebinding protection. There is no built-in authentication or multi-user authorization. If exposing it beyond a trusted local environment, supply authentication, TLS, request limits, and process resource limits in your deployment. No public hosting is configured by this repository.

### Optional diagram viewer

Add `--ui` to offer a bundled [MCP Apps viewer](docs/mcp-apps.md) to compatible
hosts. It previews inline diagrams, supports pan/zoom and complete-source
rerenders, and offers PNG/source downloads when the host advertises them.
The flag defaults to off. Ordinary tool names, results, and stateless HTTP
behavior are unchanged; no Node runtime, website, or download store is needed.

```sh
./d2-mcp --transport http --host 127.0.0.1 --port 8080 --ui
```

Connect to `/mcp` using the client's custom MCP configuration. A remote host
needs your operator-provided HTTPS endpoint, not a marketplace listing.
Client availability and download support vary; see the guide's evidence and
limitations. This feature is unreleased and is not in the published 1.2.0 binary.

## Tools

- `compile-d2`: validate syntax, semantics, layout, and server limits, with located diagnostics when available.
- `render-d2`: render one diagram and return its artifact.
- `fetch_d2_cheat_sheet`: retrieve the bundled reference and supported subset.

Both diagram tools require **exactly one** of `code` or `file_path`. `file_path` is a relative `.d2` path inside an explicitly configured `--input-dir`; it refers to the **server's** filesystem. Agents without local files should send `code`. Input is never fetched from a URL.

Example `tools/call` parameters:

```json
{
  "name": "render-d2",
  "arguments": {
    "code": "direction: right\nclient -> api -> database\ndatabase.shape: cylinder",
    "format": "png"
  }
}
```

| Argument | Behavior |
| --- | --- |
| `layout` | `dagre`, `elk`, or `tala` |
| `theme_id` | Built-in D2 theme ID, e.g. 0 (neutral), 1 (gray), 6 (GrapeSoda) |
| `dark_theme_id` | Optional SVG dark theme; PNG rejects this option |
| `sketch` | Boolean; explicit `false` is honored |
| `padding` | Integer 0–1000; explicit zero is honored |
| `scale` | Number 0.1–4 for SVG/PNG; ASCII uses its own character grid |
| `center` | Boolean controlling SVG/PNG centering |
| `board_path` | Board-kind/name pairs, e.g. `["layers", "overview"]`; omitted selects root |
| `format` | Render only: `png` (default), `svg`, `ascii` |
| `ascii_mode` | Render only: `extended` or `standard` connectors; labels retain their source characters |
| `output_name` | Render only, with `--output-dir`: `.png`, `.svg`, or `.txt` filename, without directories |
| `overwrite` | Explicit replacement; requires `--allow-overwrite` and `output_name` |

Explicit options override `vars: {d2-config: {...}}` source settings. Otherwise D2's defaults apply: Dagre, neutral theme 0, no sketch, padding 100. Source layout settings are respected. ASCII supports the selected layout but does not show colors, sketch strokes, or other image-only styles.

### Results and errors

PNG uses standard MCP image content with `image/png`. SVG uses an embedded text resource with `image/svg+xml`, so clients can save the original XML; its `urn:sha256:...` URI identifies the returned bytes and is not a persistent download endpoint. ASCII uses text content. `structuredContent` includes `status`, effective layout/theme/sketch/padding, and artifact MIME type, byte size, SHA-256, and URI. No base64 payload is duplicated in structured output.

Tool failures return `isError: true`, a readable text explanation, and:

```json
{
  "status": "error",
  "error": {
    "code": "invalid_d2",
    "message": "input.d2:1:4: ...",
    "diagnostics": [{"path": "input.d2", "line": 1, "column": 4, "message": "..."}]
  }
}
```

Diagnostics use one-based lines and columns. Other codes include `invalid_arguments`, `unsupported_feature`, `limit_exceeded`, `cancelled`, `busy`, `write_failed`, and `render_failed`. Protocol errors such as an unknown tool remain SDK protocol errors. Native renderer failures are surfaced without silently switching format.

### Files and artifacts

Local file access is opt-in. Create the directories yourself:

```sh
mkdir -p diagrams artifacts
./d2-mcp --input-dir ./diagrams --output-dir ./artifacts
```

A request with `file_path: "architecture.d2"` may import other `.d2` files within that root. Inline source may also import from that root when configured. Absolute paths, `..` escapes, and symlinks escaping the root are rejected. Sources are snapshotted per call so validation and layout see the same imported bytes.

Without `output_name`, results stay inline. To save a local artifact, include `output_name: "architecture.svg"` and `format: "svg"`. Files use private permissions and appear at the final name only after writing completes. Exclusive publication requires filesystem hard-link support; unsupported output filesystems fail explicitly. The result includes the server-local path and a `file:` resource link. This server does not expose a download endpoint for those files. Use the inline artifact for a remote client, or arrange access outside this server.

For a trusted file pipeline that refreshes a fixed filename, start with `--output-dir ./artifacts --allow-overwrite`, then explicitly pass `overwrite: true` and `output_name` on each replacement call. The server writes a temporary file in that root and renames it over the destination; it never truncates through an existing symlink or hard link. Existing symlinks and directories are refused. Concurrent replacements are last-completed-writer wins; this is not a revision/CAS API. Atomic replacement follows the host filesystem's `os.Rename` semantics (not guaranteed atomic on non-Unix platforms).

`--write-files --output-dir ./artifacts` retains the automatic file-output mode for `file_path` calls, now with unique filenames in the output root. It does not write next to arbitrary input paths.

### Images and boards

Native shapes need no extra configuration. To use local image/icon files, start with `--asset-dir ./assets`; icon paths are relative to that root, independently of the source directory. `--allow-inline-assets` permits base64 data URLs for agents without local files. PNG, JPEG, GIF (first frame), and D2's restricted SVG subset are decoded and converted to embedded PNG, including within SVG output. Quote icon values, especially data URLs. A local example is `service: {icon: "icons/service.svg"}`. Unsupported SVG features fail explicitly; download approved remote images into the asset root yourself. The server never fetches image URLs.

Multi-board source is accepted. Each render returns one artifact: root by default, or a selected board using exact pairs such as `board_path: ["layers", "overview", "scenarios", "failure"]`. D2 resolves scenario/step inheritance. Validation and aggregate limits cover all boards, including unselected ones. Batch export, animation, and cross-board navigation are not provided.

### Limits and supported subset

The server admits at most 256 KiB of combined source, 32 source files, 32 boards including root, 1000 objects and 2000 connections across all boards, two simultaneous diagram operations, and 16 MiB per returned artifact. Layout bounds are capped at 8192 pixels per side and 16 million pixels; native PNG additionally has explicit scene, raster, asset, and work budgets. MCP input frames/bodies are capped at 2 MiB. ASCII has a separate pre-allocation ceiling of 2000 cells per side and 1 million cells, independent of image scale; legends require SVG/PNG. Calls have a cooperative 30-second timeout (`--timeout`, at most 5m). Busy calls fail immediately and can be retried.

Before compilation, UTF-8 source and imports pass conservative admission checks: at most 16 combined map/dotted-key levels, 16 import levels, 10,000 projected AST operations, and 256 KiB of expanded scalar data. Each import occurrence also counts toward a 256 KiB expanded-source budget. Imported files are frozen before compilation. Large style/class/board combinations can reach these estimates before reaching the object limit.

Use literal scalar variables and explicit objects/connections. Map/array variables, recursive substitutions, structural classes, wildcard creation of objects or connections, and `${...}` interpolation inside block strings are unsupported. Attribute-only classes and style rules remain supported, including indexed styling of existing edges. Ordinary quoted/unquoted labels can use scalar variables; `vars.d2-config` and `vars.d2-legend` remain available. These restrictions deliberately exclude some valid upstream D2 programs; expand structural templates in the client before sending them. Mathematical labels are limited to 4096 bytes each.

Timeouts are cooperative: parsing and some upstream work are not preemptible. Cancellation is checked between compilation/render stages and bounded artifact-write chunks. Operation slots remain occupied until the underlying work exits. Canceled writes remove only their private temporary file; they cannot remove another request's completed artifact. Cancellation cannot undo a completed file publication or backend write, so its successful result is retained when available. These limits are useful bounds, not process isolation or a security audit. Run untrusted workloads with operating-system memory/CPU limits.

Assets are disabled unless explicitly enabled by the server. Limits include 32 distinct images, 1 MiB each, 8 MiB combined input, 1024 pixels per side, and 8 million combined decoded pixels. Repeated SVG assets have a conservative 8 MiB expansion budget before rendering. Native shapes, text, Markdown labels, code, and mathematical labels remain available; D2 omits Markdown images. No rendering network access or ambient filesystem image loading is enabled. SVG links permit HTTP, HTTPS, mailto, and fragments; emitted SVG is checked for active content and external resource references, including CSS contributed by mathematical labels. Unsupported CSS functions, escapes, and comments fail explicitly. PNG mathematics use finite SVG import budgets. TALA seed arrays are capped at 16.

## Optional document-store adapter

The shipped binary has **no state backend**, no storage flags, and only the three core tools. Integrators can supply the small [`document.Store`](document/store.go) interface via `config.Store` when constructing the application in a custom build. This is an adapter seam, not a hosted database or an authentication implementation.

A configured store adds `store-d2` for complete source replacement with atomic compare-and-swap, and enables `document_id` plus `revision` as an alternative input to validate/render. Create with `expected_revision: ""`; update only using the exact previous revision. The backend must return a new opaque revision for every successful write. Sources stored this way must be self-contained: server-local imports and local assets are rejected. Inline assets are usable only when the server enables them. The interface contract requires authorization from request context, bounded storage, cancellation, concurrency safety, and atomic revision checks shared across replicas.

```json
{"name":"store-d2","arguments":{"document_id":"architecture","expected_revision":"","code":"client -> api"}}
```

Retain the returned ID and revision explicitly, then send both:

```json
{"name":"render-d2","arguments":{"document_id":"architecture","revision":"RETURNED_REVISION","format":"svg"}}
```

A stale revision returns `revision_conflict`; a missing ID returns `document_not_found`. Handles work only with the same backend and authorization policy. No backend is installed by the shipped binary.

## Docker

```sh
docker build -t d2-mcp .
docker run --rm -i d2-mcp
# Host access stays on loopback; container listener explicitly binds all interfaces.
docker run --rm -p 127.0.0.1:8080:8080 d2-mcp --transport http --host 0.0.0.0
# Opt-in read-only source mount:
docker run --rm -i -v "$PWD/diagrams:/data:ro" d2-mcp --input-dir /data
```

The image runs as a non-root user and includes the statically built Go server. For writable bind mounts, use `--user "$(id -u):$(id -g)"` and provision output-directory ownership. No librsvg, browser, or external layout plugin is installed.

## Development

Implementation, tests and release tooling use Go. Client manifests are static
JSON; CI uses YAML and short shell commands. No Python or Node setup is needed.

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./...
# Exercise the exact binary outside the checkout:
go build -o d2-mcp .
D2_MCP_TEST_BINARY="$PWD/d2-mcp" go test -run TestDistribution -v .
```

CI runs Go race/vet tests, native Linux/macOS/Windows artifact and MCP checks,
five target builds, checksum verification and the non-root container. Optional
browser UI and provider deployment adapters are developed in separate PRs.
