# Optional MCP Apps viewer

```sh
go build -o d2-mcp .
./d2-mcp --transport http --port 8080 --ui
```

Connect an MCP Apps client to `http://127.0.0.1:8080/mcp`. Stdio also supports
`--ui`. The flag defaults off; ordinary tool results work without a viewer.
Release 1.2.0 does not include this feature. No Node runtime is needed to run the
Go binary: it embeds the built HTML.

For VS Code, merge into `.vscode/mcp.json`:

```json
{"servers":{"d2":{"type":"http","url":"http://127.0.0.1:8080/mcp"}}}
```

For remote clients, use your deployed HTTPS endpoint and its authentication.
A remote client cannot reach your laptop's loopback address. The Cloudflare
target currently serves ordinary tools with the viewer disabled.

| Client | Connection instructions |
| --- | --- |
| ChatGPT | [Custom connection setup](https://developers.openai.com/plugins/deploy/connect-chatgpt); account/admin policy controls availability |
| Claude | [Remote connectors](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp) |
| VS Code | [MCP configuration](https://code.visualstudio.com/api/extension-guides/ai/mcp) |

These are setup references. Testing uses the official MCP Apps bridge in a
sandboxed browser harness, not live sessions in those products.

## Use

Preview SVG/PNG/ASCII, drag to pan, and use zoom/fit. Edit complete source and
press **Render diagram**; typing sends no requests. D2 defaults apply unless the
input supplies rendering options. Every render uses the existing `render-d2` tool.

File/document inputs can be previewed, but paste source to edit them. The viewer
never fetches server files or forwards file paths, document IDs, output names or
overwrite flags. Edits live in the iframe and are lost on reload unless copied
or downloaded.

**Source** downloads D2 text. **PNG** exports the decoded image through a canvas;
use the ordinary MCP result for original SVG. Downloads require host support and
approval. Cancel discards late replies; server cancellation remains cooperative.

![Viewer in the local protocol harness](images/mcp-apps.png)

## Compatibility and boundaries

The official `@modelcontextprotocol/ext-apps` bridge handles UI messaging, tool
calls and downloads. Rendering requires the host's `serverTools` capability;
downloads require `downloadFile`. Unsupported controls stay disabled.

`--ui` publishes a static `ui://d2/viewer` resource. Tool metadata is negotiated
per request. Legacy stateless HTTP before `2026-07-28` may lack capabilities on
later requests; with `--ui`, those requests receive optional metadata. Explicitly
unsupported clients do not. Current HTTP, stdio and SSE use known capabilities;
no capability cache or session store is added.

SVG is displayed as an inert image, never inserted into the document. Text uses
text-only DOM properties. The bundled page permits no network or child frames
and uses a hash-pinned script CSP. Preview limits are 256 KiB source, 2 MiB SVG,
16 MiB PNG, 8192 pixels per side and 16 megapixels; they do not guarantee bounded
arbitrary browser rendering complexity.

## Build and test

With Node 24+, from `apps/`:

```sh
npm ci --ignore-scripts
npm run build
npx playwright install chromium
go build -o ../d2-mcp ..
D2_MCP_TEST_BINARY="$(cd .. && pwd)/d2-mcp" npm test
```

For an installed browser, set `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH`. CI checks the
embedded HTML matches the source and runs browser tests against the Go server.
