# Current renderer in local Cloudflare emulation

The refreshed renderer passes the existing local Worker and Durable Object
probe with Dagre, ELK, TALA, SVG, and native PNG. This is a compute feasibility
check, not a deployed MCP service. No hosting or persistent backend is added.

On 2026-10-10, the renderer at
`e037176966f66a348d950d6031ed108d1987b92a` was compiled with Go 1.27.2's standard
`js/wasm` target into a temporary copy of the
[existing experiment](https://github.com/h0rv/d2-mcp/tree/454e343534d4017592d4ac5aba6786cafddbfd4a/experiments/cloudflare).
Its bridge, Worker, probe, and lockfile were unchanged. The bridge imports this
repository's `d2.Prepare` and `d2.Render`; it is not the Go MCP HTTP server.
The pinned tools are Wrangler 4.149.0, Miniflare 5.20261006.1-alpha, and workerd
1.20261006.1, running under Node 24. Node compatibility is disabled inside workerd.
The original experiment checkout and mounted Philly repository were untouched.

The recheck found native PNG setup unnecessarily asked for the current working
directory, which is unavailable in workerd. The resolver now uses an explicit
root base; prepared image assets are already normalized inline PNGs and network
access remains disabled. A native regression also removes its working directory
and decodes the resulting PNG. This change supplies no Worker-specific API.

## Reproduction and measurements

Copy `worker.js`, `probe.mjs`, `package.json`, `package-lock.json`, and
`bridge/main.go` from the pinned experiment to a temporary directory. Install its
locked development dependencies there. With this checkout as the Go module
working directory, build the copied bridge:

```sh
# PROBE_DIR is the absolute path to that temporary copy.
mkdir -p "$PROBE_DIR/build"
GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \
  -o "$PROBE_DIR/build/d2.wasm" "$PROBE_DIR/bridge/main.go"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$PROBE_DIR/build/wasm_exec.js"
cd "$PROBE_DIR"
node probe.mjs
node probe.mjs do
```

The checked run reused the experiment's existing pinned dependencies and copied
the toolchain's unmodified loader and license. The fresh-object run starts a
separate workerd instance and sends every request through the Durable Object.
Both runs passed all 13 cases, including malformed-source HTTP 422 followed by
successful rendering. Raw observations are in
[worker.json](cloudflare-results/worker.json) and
[do.json](cloudflare-results/do.json).

| Case | Worker wall ms | Worker sampled active ms | Fresh object wall ms | Object sampled active ms |
| --- | ---: | ---: | ---: | ---: |
| Cold Dagre, including Go initialization | 1020.5 | 990.0 | 727.4 | 705.5 |
| First warm Dagre | 32.1 | 20.0 | 26.4 | 15.5 |
| First ELK | 154.1 | 150.2 | 161.5 | 179.3 |
| First TALA | 760.3 | 755.6 | 712.3 | 706.1 |
| 25-node chain | 84.5 | 83.7 | 55.9 | 35.9 |
| 100-node grid | 100.0 | 92.2 | 68.7 | 66.8 |
| Native PNG | 225.5 | 219.5 | 156.1 | 153.7 |
| Final three warm Dagre calls | 12.5–14.8 | 7.8–9.0 | 13.3–18.0 | 8.9–16.2 |

The Wasm file is 56,220,788 bytes, SHA-256
`195d68dd948dd79ea9cb4a8596f40c91f558270630a127c62e70c3c5509ca216`.
Maximum observed Wasm linear memory was 45.5 MiB; the largest post-request JS
heap sample was 2.55 MiB. Go heap statistics are inside Wasm memory and must not
be added again. The refreshed full Wrangler bundle and global startup profile
were not remeasured.

Wall time is measured outside the Worker. Sampled active time sums workerd V8
profiler samples, including GC, and covers a slightly wider interval than the
request timer. It is not billing telemetry or an exact CPU measurement. Sampling,
JIT, inspector overhead, and host CPU affect these small samples. Post-request JS
heap and Wasm memory are not peak total isolate memory. This is not a throughput,
long-lived memory, concurrency, or maximum-admitted-diagram benchmark.

## Hosting and compatibility limits

Cloudflare currently documents a 64 MiB uncompressed bundle limit, 128 MB isolate
memory, one second of global-scope startup, and 10 ms CPU per free Worker HTTP
request. The Wasm component alone is below the bundle ceiling; deployed admission
requires checking the whole bundle and runtime. Cold initialization and several
ordinary layouts have sampled active times above 10 ms locally; these measurements
do not establish reliable rendering within the free Worker CPU budget.
[Workers limits](https://developers.cloudflare.com/workers/platform/limits/).

Durable Objects have a 30-second default invocation CPU budget, configurable up
to five minutes. A compute adapter is plausible from these measurements, but
local emulation does not prove deployed quota, startup, memory, or billing
behavior. A separately authorized deployment would be needed to establish those.
[Durable Object limits](https://developers.cloudflare.com/durable-objects/platform/limits/).

The bridge uses complete source/options with no application storage reads or
writes. It denies imports and exposes only code/layout/format. It does not exercise
MCP negotiation, HTTP authentication, cancellation, source assets, board selectors,
file output, or a document backend. A production adapter and its operating policy
remain separate scope. No account access, deployment, paid resource, credential,
or permission change was made.

The old experiment's compatibility table describes its earlier renderer commit.
For this PR, the [current compatibility matrix](compatibility.md) supersedes that
table: bounded local/inline assets, board selection, and explicit file replacement
are implemented. They are tested by the native server suite, not this narrow
Cloudflare bridge. Stateless transport alone does not provide persistent state;
the optional document-store interface remains disabled by default with no backend.
