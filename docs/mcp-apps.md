# Optional MCP Apps viewer

![D2 viewer rendering a TALA diagram in the local protocol harness](images/mcp-apps.png)

Screenshot from the Chromium protocol harness with a real Go render; it is not
a ChatGPT, Claude or VS Code screenshot.

Start a refreshed binary with `--ui` to attach a portable diagram viewer to
`render-d2`. The default remains the three ordinary MCP tools. The viewer is a
single embedded HTML resource, served through MCP; it does not need a separate
website, JavaScript server, artifact database, or public download endpoint.

```sh
go build -o d2-mcp .
./d2-mcp --transport http --host 127.0.0.1 --port 8080 --ui
```

The endpoint is `http://127.0.0.1:8080/mcp`. Stdio and legacy SSE can also use
`--ui` when their host supports MCP Apps. Release 1.2.0 does not contain this
feature. Building this checked-out branch needs Go, but does not need Node;
the reviewed HTML build is committed and embedded in the executable.

## Connect directly

For VS Code, merge this into `.vscode/mcp.json`:

```json
{
  "servers": {
    "d2": {"type": "http", "url": "http://127.0.0.1:8080/mcp"}
  }
}
```

For remote clients, the operator must first supply an accessible HTTPS MCP URL,
for example `https://YOUR_SERVER/mcp`. Add that URL through the client's custom
MCP/connector settings. A remote service cannot reach your laptop's loopback
address. Publication in a marketplace or MCP registry is not required for a
custom connection; account eligibility and administrator policy still apply.

The following is **documentation evidence checked on 2026-10-12**, not an
end-to-end certification of this app in any product:

| Host | Documented connection and UI path | Limits to check |
| --- | --- | --- |
| ChatGPT | [Standard MCP Apps UI](https://developers.openai.com/plugins/build/chatgpt-ui) and [custom connection setup](https://developers.openai.com/plugins/deploy/connect-chatgpt) | [Developer-mode availability](https://help.openai.com/en/articles/12584461-developer-mode-and-mcp-apps-in-chatgpt) varies by plan/admin policy. Do not assume all accounts or mobile clients can add and use it. |
| Claude | [Custom remote connectors](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp) and [interactive connectors](https://support.claude.com/en/articles/13454812-use-interactive-connectors-in-claude) | UI is documented across web, desktop and mobile, but available host capabilities and organizational policy can differ. |
| VS Code | [Direct MCP configuration and MCP Apps](https://code.visualstudio.com/api/extension-guides/ai/mcp) | The viewer uses inline display; it does not require fullscreen, automatic chat submission, or a VS Code extension. |
| Other MCP clients | Existing `render-d2`, `compile-d2`, and `fetch_d2_cheat_sheet` tools | Ordinary artifacts work without an Apps UI. Interactive rendering/download controls require separately advertised host capabilities. |

Cloudflare is one possible operator-managed endpoint route. Its
[remote MCP guide](https://developers.cloudflare.com/agents/model-context-protocol/guides/remote-mcp-server/)
describes Streamable HTTP hosting. This change adds no Cloudflare adapter,
deployment, account, credentials, billing configuration, or storage. A full D2
renderer has compute/memory requirements beyond a thin transport example.
Choose suitable compute and apply TLS, authentication where appropriate, abuse
limits and process budgets before exposing a server. A separate hosting proof
of concept does not establish that this UI is deployed or product-tested.

## What the viewer does

The host sends the original tool input and ordinary result. The viewer shows
inline SVG or PNG in an image element, or ASCII as plain text. Drag to pan and
use the zoom/fit controls. Edit the complete D2 source, select Dagre/ELK/TALA and
SVG/PNG/ASCII, then press **Render diagram**. Each click invokes the existing
`render-d2` tool with complete source; typing makes no server calls.

Known numeric/boolean rendering settings, ASCII mode and board selection are
retained from the input. The viewer never forwards file paths, document IDs,
output names or overwrite flags. For an original server-file/document input,
the artifact can be previewed but source must be pasted to edit it. The UI does
not fetch server files or retrieve persistent documents. File-only output
links remain available in the ordinary tool result and are not fetched by the
viewer. Remote imports/assets retain the server's existing restrictions.

Cancel stops waiting and discards late replies. Cancellation is cooperative;
a host or older transport can finish already dispatched server work. Only the
latest active render can replace the preview. Errors are shown as text, and
source remains available to correct and rerender.

When the host advertises `downloadFile`, **Source** requests `diagram.d2` as
plain text and **PNG** exports the current decoded image through a canvas as
`diagram.png`. SVG previews export as PNG deliberately: untrusted vector
markup is never handed out as an active download by the UI. Use the ordinary
MCP artifact to obtain the original SVG. The host controls approval and file
delivery; denied/cancelled requests are reported. Without that capability,
download buttons are disabled. There is no browser-download or temporary-URL
fallback. ASCII remains available in the ordinary text result.

Edits live only in the iframe. Closing/reloading it can lose them; use Source
download or copy the text. Stateless HTTP does not forbid external application
state, but this viewer adds no persistence or server editing API.

## Protocol and trust boundaries

The implementation follows the
[MCP Apps specification](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx):

- Tool metadata links `_meta.ui.resourceUri` to `ui://d2/viewer`.
- The resource MIME type is `text/html;profile=mcp-app`. It has no templated path
  or caller-selected content. Enabling `--ui` makes this static resource
  enumerable even for ordinary clients.
- Known client capabilities must advertise `io.modelcontextprotocol/ui` with
  that MIME type before the tool link is offered. Metadata is copied per
  request, so concurrent clients cannot change each other's tool descriptors.
- **Legacy stateless HTTP exception:** protocols before `2026-07-28` do not
  carry capabilities on every request, and the SDK does not retain initialize
  state. When capabilities are genuinely unavailable on such an HTTP request,
  explicit `--ui` permits publishing the optional metadata. Older hosts can
  ignore it. Known unsupported capability sets omit it. Current HTTP, stdio
  and SSE use negotiated capabilities. No capability cache or session store is
  introduced to bridge this compatibility gap.
- The official `@modelcontextprotocol/ext-apps` 2.0.3 `App` bridge performs the
  UI handshake and tool/download calls. Handlers are installed before connect.
  Rendering requires `serverTools`; downloads require `downloadFile`.

The SDK accepts messages only from its parent window and validates JSON-RPC
messages. Its wildcard target origin is necessary for opaque sandbox origins;
messages target that one parent window, not other frames. The host remains the
authority that routes tools and enforces the sandbox. A malicious parent host
is not an isolation boundary the app can defend against.

Resource metadata requests empty connect/resource/frame/base-origin allowlists.
The HTML adds a hash-pinned script CSP, no network connections, no child frames,
no objects/forms, and only data images. JavaScript and styles are bundled;
there are no CDN requests. Source, diagnostics and ASCII use text-only DOM
properties. SVG is parsed off-document to establish a bounded intrinsic size,
then decoded as an inert image; it is never inserted into the live document.
Image references are cleared on replacement, cancellation and teardown. Data
images work with the standard host CSP; the viewer does not require `blob:` to
be added to the host's image policy.

Browser preview limits are intentionally separate from server limits: 256 KiB
UTF-8 source, 2 MiB SVG, 16 MiB PNG, at most 8192 pixels per side and 16 megapixels.
Size/header checks and SVG declaration rejection happen before decoding;
decoder failures are reported. These checks and browser isolation are defense
in depth, not a complete browser resource-accounting guarantee or a
security-audit claim.

## Build and verify

UI maintainers need Node 24 or newer. Dependencies and the generated asset are
pinned by `apps/package-lock.json`; the Go executable still has no Node runtime
dependency. From `apps/`:

```sh
npm ci --ignore-scripts
npm run build
npx playwright install chromium
go build -o ../d2-mcp ..
D2_MCP_TEST_BINARY="$(cd .. && pwd)/d2-mcp" npm test
```

For an already installed browser, set
`PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/absolute/path/to/chromium`. Tests exercise
the standard bridge inside a sandboxed browser iframe; they do not automate
ChatGPT, Claude or VS Code. Go integration tests cover negotiation, the embedded
resource and unchanged tools over stdio, SSE and stateless HTTP. CI rebuilds the
HTML and checks it matches the committed artifact before testing.

The earlier [`apps-sdk` branch at `8860806`](https://github.com/h0rv/d2-mcp/tree/8860806)
used ChatGPT-specific `window.openai` APIs, `text/html+skybridge`, and a temporary
server-side download map. This implementation uses the standard bridge,
preserves normal tool results and leaves downloads to the host. It is a
separate optional UI change above the core refresh, not a revival of that
branch's stateful download service.
