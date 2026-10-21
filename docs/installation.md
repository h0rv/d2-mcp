# Install d2-mcp 2.0.0

## Published binaries

[Release 2.0.0](https://github.com/h0rv/d2-mcp/releases/tag/2.0.0) includes D2
0.9.0, Dagre, ELK, TALA and native PNG rendering. No Go compiler, D2 executable,
browser or `rsvg-convert` is needed.

Download the matching binary and
[SHA256SUMS](https://github.com/h0rv/d2-mcp/releases/download/2.0.0/SHA256SUMS)
into a new directory. Checksums verify the downloaded bytes, not signed provenance.

| Platform | Download |
| --- | --- |
| Linux x86-64 | [d2-mcp-linux-amd64](https://github.com/h0rv/d2-mcp/releases/download/2.0.0/d2-mcp-linux-amd64) |
| Linux ARM64 | [d2-mcp-linux-arm64](https://github.com/h0rv/d2-mcp/releases/download/2.0.0/d2-mcp-linux-arm64) |
| macOS Intel | [d2-mcp-darwin-amd64](https://github.com/h0rv/d2-mcp/releases/download/2.0.0/d2-mcp-darwin-amd64) |
| macOS Apple Silicon | [d2-mcp-darwin-arm64](https://github.com/h0rv/d2-mcp/releases/download/2.0.0/d2-mcp-darwin-arm64) |
| Windows x86-64 | [d2-mcp-windows-amd64.exe](https://github.com/h0rv/d2-mcp/releases/download/2.0.0/d2-mcp-windows-amd64.exe) |

Windows ARM64 has no native release binary.

For Linux/macOS, choose the filename from the table:

```sh
asset=d2-mcp-linux-amd64
base=https://github.com/h0rv/d2-mcp/releases/download/2.0.0
curl --fail --location "$base/$asset" --output "$asset"
curl --fail --location "$base/SHA256SUMS" --output SHA256SUMS
# Linux:
sha256sum --check --ignore-missing SHA256SUMS
# macOS (instead of sha256sum):
shasum --algorithm 256 --check --ignore-missing SHA256SUMS
```

Continue only when your downloaded binary is reported `OK`. Then run:

```sh
chmod +x "$asset"
"./$asset" --version
```

For Windows, download and verify in PowerShell:

```powershell
$ErrorActionPreference = 'Stop'
$asset = 'd2-mcp-windows-amd64.exe'
$base = 'https://github.com/h0rv/d2-mcp/releases/download/2.0.0'
Invoke-WebRequest "$base/$asset" -OutFile $asset
Invoke-WebRequest "$base/SHA256SUMS" -OutFile SHA256SUMS
$expected = ((Select-String -SimpleMatch "  $asset" SHA256SUMS).Line -split ' ')[0]
if ((Get-FileHash $asset -Algorithm SHA256).Hash -ne $expected) { throw 'SHA-256 mismatch' }
& ".\$asset" --version
```

The version output should be `d2-mcp 2.0.0 (D2 0.9.0)`. Use the verified binary's
absolute path in your client. Optionally rename it to `d2-mcp` (`d2-mcp.exe` on
Windows) and place it on PATH after reviewing any existing installation.

## Build from the release source

With Go 1.27.2+:

```sh
go install -ldflags='-X main.version=2.0.0' github.com/h0rv/d2-mcp@2.0.0
d2-mcp --version
```

Put `$(go env GOPATH)/bin` on PATH (PowerShell: `$(go env GOPATH)\bin`), or use
your configured `GOBIN`. The explicit version flag matches the release build;
without it, the source reports its development version. The Git tag is `2.0.0`,
without a `v` prefix.

To build from a checkout instead:

```sh
git clone --branch 2.0.0 --depth 1 https://github.com/h0rv/d2-mcp.git
cd d2-mcp
go build -trimpath -ldflags='-X main.version=2.0.0' -o d2-mcp .
```

## Run

```sh
d2-mcp --transport stdio
# Or serve stateless HTTP on loopback:
d2-mcp --transport http --port 8080
```

Connect to `http://127.0.0.1:8080/mcp`. For remote use, supply TLS and
authentication; the native server has no built-in authentication.
See [client commands](agent-installation.md) and
[migration from 1.2.0](compatibility.md#upgrading-from-120-to-200).
