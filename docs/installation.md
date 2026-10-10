# Install without a source checkout

End users need a prebuilt `d2-mcp` executable and their client's MCP configuration.
They do not need this repository, Go, or the package generator. Agent skills and
prompts are optional; see [direct GitHub client installation](agent-installation.md).

## Available release

The latest published release checked on 2026-10-10 is
[1.2.0](https://github.com/h0rv/d2-mcp/releases/tag/1.2.0). It predates this refresh:
it has D2 0.7.1, older options, and requires `rsvg-convert` for PNG. The refreshed
native PNG renderer and client packages still require an authorized new release.
Nothing in this PR publishes one.

The published assets are raw executables:

| Platform | Asset |
| --- | --- |
| Linux x86-64 | `d2-mcp-linux-amd64` |
| Linux ARM64 | `d2-mcp-linux-arm64` |
| macOS Intel | `d2-mcp-darwin-amd64` |
| macOS Apple Silicon | `d2-mcp-darwin-arm64` |
| Windows x86-64 | `d2-mcp-windows-amd64.exe` |

Release 1.2.0's Linux amd64 executable requires glibc 2.34. Future release builds in this
branch use `CGO_ENABLED=0`. Windows ARM64 and 32-bit targets are not offered.

For an existing Linux x86-64 installation path, run this from any empty directory
in a POSIX shell. It downloads only the pinned official binary and checks the
digest obtained from GitHub's release metadata before making it executable:

```sh
curl --fail --location --proto '=https' --proto-redir '=https' \
  --output d2-mcp-linux-amd64 \
  https://github.com/h0rv/d2-mcp/releases/download/1.2.0/d2-mcp-linux-amd64
printf '%s  %s\n' 6ba0fb7f79820bb1ab875414e127bd59a42719e9a73c277ca1763b4898df8b12 d2-mcp-linux-amd64 \
  | sha256sum --check --status && chmod 755 d2-mcp-linux-amd64
```

Use the absolute path to that file as the client's `command`. Choose SVG with
`"args": ["--image-type", "svg"]` if using 1.2.0 without its PNG converter. The
[release API](https://api.github.com/repos/h0rv/d2-mcp/releases/tags/1.2.0) exposes
each platform asset's SHA-256 digest. On macOS use `shasum -a 256`; on Windows use
PowerShell `Get-FileHash -Algorithm SHA256` and compare the exact digest before
running the downloaded executable. A checksum from this repository/GitHub checks
integrity under that trust model; it is not independent publisher signing.

## Installer prepared for the next release

This branch adds a single standalone `install.py`, to be attached to the next
release with `SHA256SUMS`. It needs Python 3.9+ and no third-party modules. No
source checkout, compiler, administrator access, or client-settings generator is
needed. Download the script from the **chosen published release**, inspect it,
then run it with that exact release tag:

```sh
curl --fail --location --proto '=https' --proto-redir '=https' \
  --output install.py \
  https://github.com/h0rv/d2-mcp/releases/download/RELEASE_TAG/install.py
# Inspect install.py before running it. Replace RELEASE_TAG with the chosen tag.
python3 install.py --version RELEASE_TAG
```

On Windows, download `install.py` from that release in the browser and run
`py -3 install.py --version RELEASE_TAG`. Release 1.2.0 has no installer asset:
the download example becomes available only after a release containing this
change is published. A reviewed copy of the new script can also install 1.2.0,
but doing so still installs its legacy behavior.

The installer selects the platform, fetches metadata only for the exact tag in
`h0rv/d2-mcp`, and verifies the executable's size and SHA-256 before installation.
It refuses missing checksums, unsupported platforms, and existing destinations
unless `--replace` is explicit. Use `--sha256 EXPECTED_DIGEST` to pin the binary
independently of mutable release metadata; that mode skips the metadata API and
uses the supplied checksum with a fixed download-size ceiling. It does not run the binary, alter
shell startup files, or edit existing client settings.

It prints ready-to-use MCP configuration containing the absolute installed path.
The default is the common `mcpServers` JSON format. Use `--client codex`,
`--client vscode`, or `--client pi` for their formats. Merge the printed entry into
the relevant client configuration; avoid replacing a file containing other
servers. `--dir` chooses a user-owned destination. For a later upgrade, rerun
with the new exact `--version` and `--replace`.

The default destination is `~/.local/bin` on Linux/macOS or
`%LOCALAPPDATA%\d2-mcp\bin` on Windows. The printed absolute path works without
changing `PATH`. Native plugins using the bare `d2-mcp` command need that directory
on the client's `PATH`; restart the client after changing it.

## Remote server

Once an operator deploys a refreshed server, a remote client needs only its MCP
endpoint configuration. It needs no executable, Python, source, or local diagram
files. For example, where the client accepts `mcpServers`:

```json
{"mcpServers":{"d2":{"type":"http","url":"https://YOUR_SERVER/mcp"}}}
```

Replace the example with an actual operator-provided endpoint. This repository
does not supply a hosted endpoint. Send complete D2 in `code`; optional document
handles require a separately configured backend.
