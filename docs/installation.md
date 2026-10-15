# Install d2-mcp

## Current source

The refreshed renderer is not released yet. With Go 1.27.2+:

```sh
go install github.com/h0rv/d2-mcp@codex/d2-stateless-refresh
d2-mcp --version
```

Put `$(go env GOPATH)/bin` on PATH (PowerShell: `$(go env GOPATH)\bin`).
From a checkout, `go build -o d2-mcp .` builds the same server.

## Published binary — no source or compiler

Download the matching binary and [1.2.0 checksums](checksums/1.2.0.txt) into a
new directory; save the checksums as `SHA256SUMS`. These fixed hashes come from
GitHub's published asset digests. Future releases include `SHA256SUMS` directly.
Checksums verify the downloaded bytes, not signed provenance.

| Platform | Download |
| --- | --- |
| Linux x86-64 | [d2-mcp-linux-amd64](https://github.com/h0rv/d2-mcp/releases/download/1.2.0/d2-mcp-linux-amd64) |
| Linux ARM64 | [d2-mcp-linux-arm64](https://github.com/h0rv/d2-mcp/releases/download/1.2.0/d2-mcp-linux-arm64) |
| macOS Intel | [d2-mcp-darwin-amd64](https://github.com/h0rv/d2-mcp/releases/download/1.2.0/d2-mcp-darwin-amd64) |
| macOS Apple Silicon | [d2-mcp-darwin-arm64](https://github.com/h0rv/d2-mcp/releases/download/1.2.0/d2-mcp-darwin-arm64) |
| Windows x86-64 | [d2-mcp-windows-amd64.exe](https://github.com/h0rv/d2-mcp/releases/download/1.2.0/d2-mcp-windows-amd64.exe) |

**1.2.0 is the older renderer**, without TALA or the new options. Use
`--image-type svg` unless `rsvg-convert` is installed for PNG. Its Linux amd64
binary requires glibc 2.34. Windows ARM64 has no native release binary.

The published Linux 1.2.0 binary also writes D2 diagnostic output to stdout
during rendering, which breaks strict MCP stdio clients. The current source
fixes this, but updated binaries are not published yet. Prefer the
[hosted connection](agent-installation.md#remote-http) or current source.

Verify in the download directory **before running or replacing anything**:

```sh
# Linux:
sha256sum --check --ignore-missing SHA256SUMS
# macOS:
shasum --algorithm 256 --check --ignore-missing SHA256SUMS
```

Continue only when the downloaded binary is reported `OK`. On Linux/macOS,
make that file executable with `chmod +x d2-mcp-<platform>` and use its absolute
path in your client. Optionally rename it to `d2-mcp` and place it on PATH after
reviewing any existing installation.

For Windows, verify in PowerShell:

```powershell
$asset = 'd2-mcp-windows-amd64.exe'
$expected = ((Select-String -SimpleMatch "  $asset" SHA256SUMS).Line -split ' ')[0]
if ((Get-FileHash $asset -Algorithm SHA256).Hash -ne $expected) { throw 'SHA-256 mismatch' }
```

Use the verified `.exe`'s absolute path in your client, or rename it to
`d2-mcp.exe` and add its directory to PATH. No installer changes settings or
overwrites an existing binary.

## Run

```sh
d2-mcp --transport stdio
# Or serve stateless HTTP on loopback:
d2-mcp --transport http --port 8080
```

Connect to `http://127.0.0.1:8080/mcp`. For remote use, supply TLS and
authentication; the native server has no built-in authentication.
See [client commands](agent-installation.md).
