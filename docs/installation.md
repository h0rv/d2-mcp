# Install d2-mcp

## Current source

The refreshed binary is not released yet. With Go 1.27.2+, install this branch:

```sh
go install github.com/h0rv/d2-mcp@codex/d2-stateless-refresh
d2-mcp --version
```

Put `$(go env GOPATH)/bin` on PATH (PowerShell: `$(go env GOPATH)\bin`).
From a checkout, `go build -o d2-mcp .` builds the same server.

## Published binary

With [uv](https://docs.astral.sh/uv/getting-started/installation/) 0.13.0+, this
command works in a POSIX shell or PowerShell without a checkout or compiler:

```sh
uv run https://raw.githubusercontent.com/h0rv/d2-mcp/5f547a485435a4797ffc4e2cf0e27f6c4c4bac18/scripts/install.py --version 1.2.0
```

It verifies the platform binary's SHA-256 and prints client configuration. The
installer is pinned to a reviewed commit; **1.2.0 is the older renderer**, without
TALA or the new options. Use `--image-type svg` with that binary unless
`rsvg-convert` is installed for PNG. Linux amd64 requires glibc 2.34.

| Platform | Supported architecture | Default directory |
| --- | --- | --- |
| Linux | amd64, arm64 | `~/.local/bin` |
| macOS | Intel, Apple Silicon | `~/.local/bin` |
| Windows | amd64 | `%LOCALAPPDATA%\d2-mcp\bin` |

Use the printed absolute path in your client, or add that directory to its PATH.
Add `--client codex`, `--client vscode`, or `--client pi` for its config format;
`--dir PATH` selects a directory and `--replace` explicitly upgrades an existing
binary. Merge the entry into existing settings. No settings or PATH are edited.

## Run

```sh
d2-mcp --transport stdio
# Or serve stateless HTTP on loopback:
d2-mcp --transport http --port 8080
```

Connect to `http://127.0.0.1:8080/mcp`. For remote use, supply TLS and
authentication; the native server has no built-in authentication.
See [client commands](agent-installation.md) and [Cloudflare](cloudflare.md).
