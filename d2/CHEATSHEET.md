# D2 0.9.0 quick reference for d2-mcp

Use `compile-d2` to validate a complete source, then `render-d2` to obtain PNG, SVG, or ASCII. Send exactly one of `code` or `file_path`. Files and imports need an explicit server `--input-dir`; inline `code` works without local files. Render settings are per call: `layout`, `theme_id`, `dark_theme_id` (SVG), `sketch`, `padding`, `scale`, `center`, `format`, and `ascii_mode`.

Dagre, ELK, and TALA are bundled, including TALA's open-source layout. Defaults match D2: Dagre, theme 0, sketch false, padding 100. Explicit tool options take precedence over source configuration. Use theme 6, sketch true, and padding 5 for the previous d2-mcp appearance.

## Basic graph

```d2
direction: right
client: Client
api: API service
db: Database {shape: cylinder}
client -> api: HTTPS
api -> db: query
```

Declare nodes with identifiers, use `id: Label` for display labels, and use `->`, `<-`, `<->`, or `--` for connections. D2 identifiers are case-insensitive. Quote identifiers that contain punctuation or reserved words.

## Containers and styling

```d2
cloud: {
  api: Gateway {shape: hexagon}
  workers: {
    a: Worker A
    b: Worker B
  }
  api -> workers.a
  api -> workers.b
}
cloud.style.fill: "#eef4ff"
cloud.workers.*.style.border-radius: 12
```

Common shapes include rectangle, square, circle, oval, diamond, hexagon, cylinder, queue, page, package, cloud, text, class, sql_table, and sequence_diagram. Set `width` and `height` explicitly when useful; layout engine support varies for advanced positioning. The compiler returns an error when a chosen layout cannot satisfy a constraint.

## Source configuration and variables

```d2
vars: {
  primary: "#336699"
  d2-config: {
    layout-engine: tala
    theme-id: 0
    sketch: false
    pad: 40
  }
}
api: {style.fill: ${primary}}
api -> database
```

Useful light themes include 0 (neutral), 1 (gray), 3 (cool classics), and 6 (GrapeSoda). Theme IDs are validated against the bundled D2 catalog. TALA accepts `vars.d2-config.data.tala-seeds` within the nested `vars` map, with at most 16 seeds in this server.

## Markdown, code, and math

```d2
notes: |md
  # Service contract
  - **Retry** transient errors
  - Keep requests idempotent
|
implementation: |go
  func add(a, b int) int { return a + b }
|
formula: |latex
  x^2 + y^2 = z^2
|
```

PNG is native Go; no browser or converter is needed. D2 omits Markdown images. Icons and image assets are disabled here, including data URLs. Use native shapes instead. Links in SVG may use HTTP, HTTPS, mailto, or a fragment; the server never fetches a source URL.

## Sequence diagrams

```d2
shape: sequence_diagram
client -> api: Request
api -> database: Query
database -> api: Result
api -> client: Response
```

## Tables

```d2
users: {
  shape: sql_table
  id: int {constraint: primary_key}
  name: text
}
orders: {
  shape: sql_table
  id: int {constraint: primary_key}
  user_id: int {constraint: foreign_key}
}
orders.user_id -> users.id
```

## Imports and results

A rooted file may use `...@common.d2` to import another `.d2` file under the configured input directory. Paths are relative; neither absolute paths nor escaping symlinks are allowed. Imports are snapshotted per request. Complete stored documents in optional storage adapters cannot use local imports.

PNG returns MCP image content; SVG returns an embedded text resource containing the XML; ASCII returns text. Structured results include artifact size, SHA-256, MIME type, URI, and effective settings. `output_name` writes a new file only when `--output-dir` is configured. Never assume returned resource URIs are persistent download endpoints.

ASCII `extended` uses Unicode box drawing; `standard` uses basic ASCII connectors. Source labels retain their own characters. Image styling, color, and sketch strokes do not appear in ASCII. Keep labels short; the server bounds the ASCII canvas before allocation. Legends require SVG or PNG.

Only single-board diagrams are supported. Submit each desired layer/scenario/step as a separate complete source. Limits include 256 KiB combined source, 32 files, 1000 objects, 2000 connections, two simultaneous operations, and a cooperative timeout. Reduce diagrams after `limit_exceeded`; retry `busy` errors later. Tool errors have `isError: true` plus a structured error code and located diagnostics when available.

HTTP calls do not depend on prior sessions. Carry source/options explicitly. If an integrator configures the optional store, retain and pass `document_id` and `revision`; arbitrary tool metadata is not automatically round-tripped by clients. IDs only work with the same shared store and authorization policy.

More syntax: [D2 tour](https://d2lang.com/tour). Upstream: [d2lang/d2](https://github.com/d2lang/d2).
