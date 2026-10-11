# Install for an agent

The optional package supplies a portable `d2` skill, two prompts, and native client configuration. Install the server binary separately using the [installation guide](installation.md). Basic client setup needs an installed binary, Git, and the client; it needs no source checkout, Go compiler, Python, or package generator.

**Release status:** these manifests and the refreshed server are introduced by this PR. The commands below select the `codex/d2-stateless-refresh` branch after its manifests are pushed; they do not assume the default branch already contains them. Use a reviewed tag or commit containing the package when one is available. Published **1.2.0** supports basic MCP, but does not gain TALA, native PNG, or the new tool options by installing this skill. The full workflow requires a refreshed binary; none is published by this PR. The skill checks advertised schemas before selecting features.

## Claude Code

Make `d2-mcp` available on the **PATH of the process launching Claude Code**, then run from any directory:

```sh
d2-mcp --help
claude plugin marketplace add 'h0rv/d2-mcp#codex/d2-stateless-refresh'
claude plugin install d2@d2-mcp
```

The plugin starts `d2-mcp --transport stdio`. It includes no installation hooks, downloads, credentials, or remote endpoint. Start a new session, inspect `/mcp`, and run `/d2:d2-diagram browser to API to database`. The skill is `/d2:d2`; `/d2:d2-check` validates source. Claude's GitHub marketplace installer fetches the package itself; do not add the raw `marketplace.json` URL, which cannot resolve this catalog's relative plugin path.

See the official [plugin manifest](https://code.claude.com/docs/en/plugins-reference), [marketplace format](https://code.claude.com/docs/en/plugins/create-marketplace), and [GitHub/ref installation](https://code.claude.com/docs/en/plugins/host-marketplace).

## Codex

With `d2-mcp` on Codex's launch PATH, use a CLI version that provides `codex plugin`:

```sh
codex plugin marketplace add h0rv/d2-mcp --ref codex/d2-stateless-refresh
codex plugin add d2@d2-mcp
```

Start a new session and choose the `d2` skill from the skill/plugin picker, or ask for a D2 diagram. The package includes the current [Agent Plugins manifest and MCP format](https://developers.openai.com/plugins/build/plugins) and the supported Codex compatibility manifest. The shared prompt files can be pasted as ordinary prompts; they are not Codex slash commands. This is a repository marketplace, not a ChatGPT cloud connector or a public plugin-directory listing.

The commands above were checked with Codex CLI **0.159.0-alpha.3**. Older clients without plugin installation can register just the server as shown below. A portable skill may also be installed in a project's `.agents/skills/d2/SKILL.md` using the [Codex skills format](https://developers.openai.com/codex/skills).

Codex plugin installation writes to the client's user configuration and plugin cache. `--cd` selects a workspace, and `--config` overrides values; neither supplies an isolated installation directory. The checked version has no plugin `--scope` or `--install-dir` option. Its [versioned plugin implementation](https://github.com/openai/codex/blob/rust-v0.159.0-alpha.3/codex-rs/cli/src/plugin_cmd.rs#L691) resolves the user directory before loading overrides, and [marketplace installation](https://github.com/openai/codex/blob/rust-v0.159.0-alpha.3/codex-rs/cli/src/marketplace_cmd.rs#L138) uses that directory. In a managed environment where it is read-only, installation needs to run in the user's normal client environment; a project config does not remove that requirement. This PR does not claim a complete Codex plugin installation was exercised in its restricted execution environment.

## Pi

Use Pi **0.99.0 or later** for native MCP; verified here with `@earendil-works/pi-coding-agent` **1.1.0** (Node 22.19+). Pi installs the shared skill and prompts from the repository's root package manifest. Configure MCP separately:

```sh
pi install git:github.com/h0rv/d2-mcp@codex/d2-stateless-refresh
pi mcp add d2 --exposure direct -- d2-mcp --transport stdio
pi mcp list --json
```

Restart Pi and run `/skill:d2`, `/d2-diagram browser to API to database`, or `/d2-check` with source. Pi packages do not install MCP configuration. Native MCP discovery works without a model session; tools receive client-specific prefixes. Older third-party MCP extensions may conflict with native support. See the versioned official [Git/package format](https://github.com/earendil-works/pi/blob/v1.1.0/packages/coding-agent/docs/packages.md), [MCP commands](https://github.com/earendil-works/pi/blob/v1.1.0/packages/coding-agent/docs/mcp.md), and [prompt templates](https://github.com/earendil-works/pi/blob/v1.1.0/packages/coding-agent/docs/prompt-templates.md).

## Direct MCP registration and remote servers

A skill/plugin is optional. For an absolute binary path, custom flags, or a client without plugins, register the server directly. Choose the command for your client instead of installing the stdio plugin, to avoid duplicate connections:

```sh
claude mcp add --scope user --transport stdio d2 -- /absolute/path/d2-mcp --transport stdio
codex mcp add d2 -- /absolute/path/d2-mcp --transport stdio
pi mcp add d2 --exposure direct -- /absolute/path/d2-mcp --transport stdio
```

The standalone installer prints mergeable client configuration using the installed absolute path and does not change PATH. This is useful when a GUI client does not inherit your shell PATH. Do not replace an existing client settings file with a snippet.

For a server you already operate, set `D2_MCP_URL` to its complete Streamable HTTP endpoint and choose your client's command:

```sh
claude mcp add --scope user --transport http d2 "$D2_MCP_URL"
codex mcp add d2 --url "$D2_MCP_URL"
pi mcp add d2 --exposure direct --url "$D2_MCP_URL"
```

No public endpoint is configured. Use HTTPS outside loopback testing. Configure authentication through the client's supported OAuth/header settings, never through committed credential-bearing URLs. The server itself has no built-in authentication. Pi can keep its separately installed skill/prompts with either connection. For Claude/Codex, a generated remote plugin is available below if you want the skill and remote connection packaged together. See [Claude MCP](https://code.claude.com/docs/en/mcp) and [Codex MCP](https://developers.openai.com/codex/mcp).

## Advanced: generate a configured bundle

Maintainers and users needing a customized plugin may generate one from a checkout. This optional workflow uses [uv](https://docs.astral.sh/uv/getting-started/installation/) 0.13.0 or newer and an existing binary or remote endpoint. Script metadata selects Python 3.11+; uv downloads a compatible interpreter if needed. No third-party Python packages are used:

```sh
uv run agents/build.py --local-command /absolute/path/d2-mcp --output ./agents/build/local
# Or, for your existing remote endpoint:
uv run agents/build.py --url "$D2_MCP_URL" --output ./agents/build/remote
```

The output directory must be new. Generation does not change client settings or contact the endpoint. HTTPS is required except on loopback; credential-bearing URLs, queries, fragments, invalid ports, control characters, and ambiguous hostnames are rejected. Use ASCII DNS/punycode or canonical IP addresses. Internationalized URL paths are supported.

Set `D2_AGENT_BUNDLE` to the absolute generated directory and choose one client installation method:

```sh
claude plugin marketplace add "$D2_AGENT_BUNDLE"
claude plugin install d2@d2-local
# Or:
codex plugin marketplace add "$D2_AGENT_BUNDLE"
codex plugin add d2@d2-local
# Or install Pi resources, then configure its MCP connection separately:
pi install "$D2_AGENT_BUNDLE/plugins/d2"
```

Keep the bundle available while installed. For temporary Claude testing, use `claude plugin validate "$D2_AGENT_BUNDLE/plugins/d2"` and `claude --plugin-dir "$D2_AGENT_BUNDLE/plugins/d2"`. Generated `configs/mcp.json`, `configs/vscode.json`, `configs/pi.json`, and `configs/codex.toml` are client-specific mergeable snippets. MCP itself defines no universal client configuration format.

To opt into server-local source and artifact directories:

```sh
mkdir -p diagrams artifacts
uv run agents/build.py --local-command /absolute/path/d2-mcp --output ./agents/build/files \
  --server-arg=--input-dir --server-arg="$PWD/diagrams" \
  --server-arg=--output-dir --server-arg="$PWD/artifacts"
```

Local bundles force stdio. This example does not enable assets or overwriting; see the main README for those opt-ins. Send complete `code` to remote servers: `file_path`, assets, and output paths refer to the server's machine. Inline artifacts work without output directories. Hash URIs and server-local paths are not hosted download URLs. Every normal request carries complete source/options; no document backend is installed.

## Maintainer checks and future endpoint configuration

Static GitHub packages always use the local `d2-mcp` command. `agents/defaults.json` is the generator's only default endpoint source. After a deployment is separately authorized and verified, its `remote_url` can supply generated remote packages. Selection is explicit `--local-command`, otherwise explicit `--url`, `D2_MCP_URL`, verified generator default URL, then local binary. Existing installations are snapshots and do not silently change connection. A deployment-specific authentication flow still needs verification before distributing that default.

```sh
D2_MCP_TEST_BINARY=/absolute/path/d2-mcp uv run agents/test_build.py -v
# Optional native Codex parsing, with no saved settings or model requests:
D2_MCP_TEST_CODEX=/absolute/path/codex uv run agents/test_build.py -v
```

Tests cover endpoint selection, URL rejection, non-destructive generation, static and generated manifest paths, matching client configurations including Unicode paths, and real stdio/loopback HTTP initialize/discover/validate/SVG-render flows. A static package also runs from a fresh directory outside the checkout with the binary on PATH and conflicting transport environment variables.

| Client | Verified without a model/account call |
| --- | --- |
| Claude Code 2.1.296 | Fresh copied root marketplace outside the checkout: native validation, install, and plugin-provided stdio MCP connection despite ambient HTTP transport |
| Codex CLI 0.159.0-alpha.3 | Native stdio/HTTP config parsing from a fresh directory, including Unicode paths, using read-only CLI overrides; full plugin installation remains blocked by the read-only user directory and lack of a separate install-location option |
| Pi 1.1.0 | Fresh copied root package outside the checkout: native install, skill and both prompts loaded without diagnostics, stdio discovery; generated packages also passed argument expansion and loopback HTTP discovery |

GitHub delivery of the new root manifests requires the PR branch to contain them; local fixture validation alone does not establish that they are published. These checks do not require model-account interaction or registry publication.
