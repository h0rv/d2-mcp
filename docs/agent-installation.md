# Install for an agent

The package contains one portable `d2` skill, two diagramming prompts, and client-specific MCP wiring. The server stays Go. Setup generates files locally; it does not modify client settings, install a binary, provision storage, or deploy anything. Python 3.11+ is needed only for package generation/tests.

## Choose the connection once

From this checkout, build the refreshed binary and generate a package in a **new** directory:

```sh
go build -o d2-mcp .
python3 agents/build.py --local-command "$PWD/d2-mcp" --output ./agents/build/local
```

For a server you already operate, supply its complete Streamable HTTP endpoint in `D2_MCP_URL`, then generate a separate package:

```sh
python3 agents/build.py --url "$D2_MCP_URL" --output ./agents/build/remote
```

No public endpoint exists yet. HTTPS is required except for loopback testing. The generator does not verify or contact the endpoint; credential-bearing URLs, query strings, fragments, invalid ports, and control characters are rejected. Configure your deployment's authentication through the client's supported OAuth/header settings; never put tokens in committed URLs or packages. The server itself does not implement authentication.

In the following commands, set `D2_AGENT_BUNDLE` to the **absolute path** of the generated `agents/build/local` or `agents/build/remote` directory. Keep it available while installed. Local packages reference the built binary by absolute path, so rebuild/regenerate if you move it. Choose one installation method per client to avoid duplicate tools.

## Claude Code

For a temporary session with the skill, prompts, and MCP connection together:

```sh
claude plugin validate "$D2_AGENT_BUNDLE/plugins/d2"
claude --plugin-dir "$D2_AGENT_BUNDLE/plugins/d2"
```

For a persistent install:

```sh
claude plugin marketplace add "$D2_AGENT_BUNDLE"
claude plugin install d2@d2-local
```

Start a new session, inspect `/mcp`, and run `/d2:d2-diagram browser to API to database`. The skill is `/d2:d2`; `/d2:d2-check` validates source. These commands use the [official Claude plugin format](https://code.claude.com/docs/en/plugins-reference) and [local marketplace installation](https://code.claude.com/docs/en/plugins/create-marketplace).

## Codex

Current Codex CLI supports local marketplaces:

```sh
codex plugin marketplace add "$D2_AGENT_BUNDLE"
codex plugin add d2@d2-local
```

Start a new session and invoke the `d2` skill from the skill/plugin picker, or ask for a D2 diagram. The generated package includes the current [Agent Plugins manifest and MCP format](https://developers.openai.com/plugins/build/plugins) plus the supported Codex compatibility manifest. It does not register a ChatGPT cloud connector or claim to be in a public plugin directory.

If your Codex version lacks plugin installation, copy `agents/package/skills/d2` into your project's `.agents/skills/d2` and merge the generated `configs/codex.toml` into your Codex config. Alternatively, register just the server with `codex mcp add d2 -- /absolute/path/d2-mcp`, or `codex mcp add d2 --url "$D2_MCP_URL"`. See [Codex skills](https://developers.openai.com/codex/skills) and [MCP configuration](https://developers.openai.com/codex/mcp). The shared prompt files can be pasted as ordinary prompts; they are not advertised as Codex slash commands.

## Pi

Use Pi **0.99.0 or later** for native MCP; this package was tested with `@earendil-works/pi-coding-agent` **1.1.0** (Node 22.19+). Install the skill/prompts, then connect MCP separately:

```sh
pi install "$D2_AGENT_BUNDLE/plugins/d2"
# Choose the matching connection:
pi mcp add d2 --exposure direct -- /absolute/path/d2-mcp
# Or: pi mcp add d2 --exposure direct --url "$D2_MCP_URL"
pi mcp list --json
```

For custom server arguments, merge generated `configs/pi.json` into Pi's MCP config instead of the `mcp add` command. Pi packages load skills/prompts, not MCP configuration. Restart Pi and run `/skill:d2`, `/d2-diagram browser to API to database`, or `/d2-check` with source. Native MCP discovery works without a model session; tools receive client-specific prefixes. Older third-party `/mcp` extensions may conflict with native support. [Official Pi packages](https://github.com/earendil-works/pi/blob/v1.1.0/packages/coding-agent/docs/packages.md), [MCP](https://github.com/earendil-works/pi/blob/v1.1.0/packages/coding-agent/docs/mcp.md), [prompt templates](https://github.com/earendil-works/pi/blob/v1.1.0/packages/coding-agent/docs/prompt-templates.md).

## Other clients and file workflows

Generated `configs/mcp.json` supplies the common `mcpServers` wrapper; `configs/vscode.json` uses VS Code's `servers` wrapper. Merge the relevant entry into existing settings rather than replacing the whole file. MCP does not specify a universal client config or plugin installer. Any compatible client can use stdio or Streamable HTTP directly without the skill package.

Send complete `code` to remote servers. `file_path`, asset roots, and output paths refer to the server's machine. To enable a local source/artifact pipeline explicitly, create its directories and generate a package with additional server flags:

```sh
mkdir -p diagrams artifacts
python3 agents/build.py --local-command "$PWD/d2-mcp" --output ./agents/build/files \
  --server-arg=--input-dir --server-arg="$PWD/diagrams" \
  --server-arg=--output-dir --server-arg="$PWD/artifacts"
```

This does not enable assets or overwriting. See the main README for those opt-ins. Returned inline artifacts work without output directories. A returned hash URI or server-local path is not a hosted download URL. No document backend is installed; every normal request carries complete source/options.

## Maintainer checks and the future remote default

`agents/defaults.json` is the only default endpoint source. After a real deployment is authorized and verified, set its `remote_url` to that endpoint and regenerate packages. Selection is explicit `--local-command`, otherwise explicit `--url`, `D2_MCP_URL`, verified default URL, then local binary. Existing installations are snapshots and do not silently switch to remote. A deployment-specific auth flow still needs verification before distributing that default.

```sh
D2_MCP_TEST_BINARY="$PWD/d2-mcp" python3 -m unittest discover -s agents -v
```

Tests cover endpoint selection, URL rejection, non-destructive generation, manifest paths, matching client configurations (including Unicode paths), and real stdio/loopback HTTP initialize/discover/validate/SVG-render flows. Additional checks used:

| Client | Verified without a model/account call |
| --- | --- |
| Claude Code 2.1.296 | Native local/HTTP manifest validation, local marketplace install, plugin-provided stdio MCP connection |
| Codex CLI 0.159.0-alpha.3 | Native stdio/HTTP config parsing and installation-command help; full plugin installation was not exercised in this environment |
| Pi 1.1.0 | Native package install, skill/prompt loading and argument expansion, stdio and loopback HTTP discovery |

Client validation does not prove a future endpoint is deployed, authorized, or reachable from every client. No model-account interaction or public registry publication is required for these checks.
