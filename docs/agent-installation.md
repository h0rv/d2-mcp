# Connect your client

[Install the binary](installation.md) first. Replace `d2-mcp` with its absolute
path if your client does not inherit your shell's PATH. Choose one connection
method to avoid registering the server twice.

## Claude Code

```sh
claude mcp add --scope user --transport stdio d2 -- d2-mcp --transport stdio
```

For the optional skill and prompts, use the plugin instead:

```sh
claude plugin marketplace add 'h0rv/d2-mcp#codex/d2-stateless-refresh'
claude plugin install d2@d2-mcp
```

Restart Claude; check `/mcp`. Plugin prompts: `/d2:d2-diagram` and `/d2:d2-check`.

## Codex

```sh
codex mcp add d2 -- d2-mcp --transport stdio
```

With a CLI supporting `codex plugin`, the optional skill installs with:

```sh
codex plugin marketplace add h0rv/d2-mcp --ref codex/d2-stateless-refresh
codex plugin add d2@d2-mcp
```

Restart Codex and choose the `d2` skill. Plugin installation needs a writable
user configuration directory; run it in your normal client environment if a
managed workspace makes that directory read-only.

## Pi

Pi 0.99+ has native MCP support. Install the optional skill/prompts and register
the server separately:

```sh
pi install git:github.com/h0rv/d2-mcp@codex/d2-stateless-refresh
pi mcp add d2 --exposure direct -- d2-mcp --transport stdio
pi mcp list --json
```

Restart Pi; use `/skill:d2`, `/d2-diagram`, or `/d2-check`.

## Claude Desktop / VS Code

Merge this entry into Claude Desktop's `claude_desktop_config.json`:

```json
{"mcpServers":{"d2":{"command":"/absolute/path/d2-mcp","args":["--transport","stdio"]}}}
```

For VS Code, put it under `servers` in `.vscode/mcp.json` instead of
`mcpServers`. On Windows use an escaped path such as `C:\\tools\\d2-mcp.exe`.

## Remote HTTP

Set `D2_MCP_URL` to your deployed HTTPS endpoint, then choose your client:

```sh
claude mcp add --scope user --transport http d2 "$D2_MCP_URL"
codex mcp add d2 --url "$D2_MCP_URL"
pi mcp add d2 --exposure direct --url "$D2_MCP_URL"
```

In PowerShell use `$env:D2_MCP_URL`. Configure authentication using the client's
OAuth/header settings, never credentials in the URL. Send complete `code`;
file paths refer to the server's machine. No hosted endpoint is supplied.

## Custom plugin configuration

From a checkout, generate a new bundle for an absolute binary path or remote URL:

```sh
uv run agents/build.py --local-command /absolute/path/d2-mcp --output ./agents/build/local
uv run agents/build.py --url "$D2_MCP_URL" --output ./agents/build/remote
```

Requires uv 0.13+ / Python 3.11+. Install the generated directory as a Claude or
Codex marketplace, or use its `configs/` snippets. The destination must be new;
generation never changes client settings. All clients share the same skill and
prompts; small manifests adapt each client's required format without symlinks.
