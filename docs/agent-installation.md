# Connect your client

For hosted use, skip installation and use [Remote HTTP](#remote-http) below.
For local use, [install the binary](installation.md) first and replace `d2-mcp`
with its absolute path if your client does not inherit your shell's PATH.
Choose one connection method to avoid registering the server twice.

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

The public endpoint is `https://d2.horv.co/mcp`; it needs no Authorization header.
No D2 binary, repository plugin, or marketplace installation is needed for these
remote connections. Choose the command for your installed client:

```sh
claude mcp add --scope user --transport http d2 https://d2.horv.co/mcp
codex mcp add d2 --url https://d2.horv.co/mcp
pi mcp add d2 --exposure direct --url https://d2.horv.co/mcp
```

Start a new Claude Code or Codex session and check `/mcp`. In current pi, start a
session or use `/reload`, then `/mcp`. These literal commands also work in
PowerShell; no shell-specific environment variable setup is required.

For ChatGPT web, if your account/workspace allows custom MCP servers, open
Plugins → + → Add custom MCP server. Name it `d2`, enter
`https://d2.horv.co/mcp` under Connection, and use no authentication. Review the
warning and discovered tools, create and install the resulting plugin, then
select `@d2` in a new chat. This is a direct custom connection, not a published
D2 marketplace listing. Current ChatGPT instructions do not require enabling
Developer Mode. Availability and creation permissions vary by account and
workspace; use the official help below if the option is missing.

Setup references verified on 2026-10-16:

- [Claude Code HTTP MCP and scope](https://code.claude.com/docs/en/mcp).
- [Codex MCP](https://developers.openai.com/codex/mcp/); `codex mcp add --help`
  also confirms the `--url` option.
- [pi MCP](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/mcp.md)
  and [CLI options](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/cli.md#mcp-commands).
- [ChatGPT connection steps and availability](https://help.openai.com/en/articles/12584461-connecting-custom-mcp-servers-in-chatgpt)
  and [connection testing](https://developers.openai.com/plugins/deploy/connect-chatgpt).

Send complete `code`; the hosted service has no file access. Your agent can read
an available project or your chat notes and send the resulting diagram source.
For your own deployment, substitute its HTTPS endpoint and
configure any authentication using the client's OAuth/header settings, never
credentials in the URL.

For an absolute binary path or custom remote endpoint, use the direct client
commands above. The optional static plugins use `d2-mcp` on PATH. All clients
share one copy of the skill and prompts; no package generator is needed.
