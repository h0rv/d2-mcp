# Local Cloudflare rendering feasibility

**D2 0.9.0 runs successfully inside local workerd as Go WebAssembly.** The actual
rendering package from PR #7 produced SVG with Dagre, ELK, and TALA, and native
PNG. The same bridge worked through a minimal SQLite-class Durable Object with
no application storage reads or writes. This is a separate experiment, not an
MCP server, production Worker, or deployment configuration recommendation.

The finding supports further investigation of a Durable Object compute adapter
or a paid Worker. It does **not** establish reliable direct rendering on the free
Worker's 10 ms request CPU budget. Cold initialization and several layouts were
far above that budget locally. No Cloudflare account access, remote bindings,
deployment, persistent application data, credentials, or paid resources were
used. The mounted Philly repository was not involved.

## Reproduce locally

Start at this directory with Go 1.27.2 and Node 24 available:

```sh
npm ci --no-audit --no-fund
npm run build
npm test
npm run profile
```

The lockfile pins Wrangler 4.149.0, its own Miniflare dependency
5.20261006.1-alpha, and workerd 1.20261006.1. The alpha version here is the one
shipped with this stable Wrangler release. The harness uses Miniflare's
exported v4-options converter and explicit module definitions.

`profile.sh` keeps Wrangler config/logs in this directory and disables telemetry.
`wrangler check startup` internally builds with `--dry-run` and profiles locally;
there is no deployment command in these scripts. Build outputs and installed
packages are ignored. `go test ./...` also passes from the repository root; the
Go bridge is excluded from native builds by its `js && wasm` build constraint.

## What was adapted

`bridge/main.go` imports the unchanged production `d2.Prepare` and `d2.Render`
functions from parent commit `59273aa89d554870721b89bcb2fead88a7a3c91b`.
`build.sh` compiles it with Go's standard `js/wasm` target and copies that exact
toolchain's unmodified `wasm_exec.js` and license. This is not a TinyGo port or
a JavaScript reimplementation of D2.

`worker.js` statically imports the precompiled Wasm module, then starts Go lazily
inside the first request. A `syscall/js` Promise bridge accepts complete source
and options and returns artifact data. It avoids dynamic byte-buffer Wasm
compilation, Web Workers, browser APIs, filesystem access, and a TCP listener.
Node compatibility is explicitly disabled. No changes to Go's runtime loader
were necessary. Starting Go during a request also places its time/crypto work
inside a permitted request context.

The Worker handler and the Durable Object call the same renderer. `/do` routes
to an object named `local-renderer`. The class never receives or accesses a
storage object. It retains warm code and memory only; each call includes its
entire source. The `do` test starts a fresh workerd instance and sends **all**
requests through the object, so its cold result does not borrow the earlier
Worker run's initialized Go runtime. Both runs use ephemeral local emulation.
An object ID here selects a compute instance, not a saved document.

The existing Go MCP SDK server is not running in this probe. A real Workers
integration would still need an HTTP/MCP adapter, concurrency and cancellation
policy, authentication, lifecycle testing, and deployment validation. Full-source
render calls are enough to test compute feasibility without adding document
storage. File imports are deliberately denied in the bridge. SVG is returned in
JSON for inspection; the PNG case verifies rendering and reports its byte count.

## Recorded results, 2026-10-10

Raw measurements are in [worker.json](results/worker.json) and
[do.json](results/do.json); representative outputs are
[Worker SVG](results/worker-example.svg) and [Durable Object SVG](results/do-example.svg).
The small fixture is `client -> api -> database` with a cylinder database.
The two larger fixtures are a 25-node chain and 100 nodes in a 10-column grid.

| Case | Worker wall ms | Worker sampled active ms | Fresh-object run wall ms | Object sampled active ms |
| --- | ---: | ---: | ---: | ---: |
| Cold Dagre SVG, including Go initialization | 1356.3 | 1340.4 | 1041.6 | 1010.3 |
| First warm Dagre SVG | 55.0 | 23.4 | 36.9 | 26.0 |
| First ELK SVG | 162.6 | 157.0 | 167.8 | 130.1 |
| First TALA SVG | 770.4 | 763.4 | 649.9 | 623.2 |
| 25-node SVG | 78.7 | 72.8 | 53.3 | 44.5 |
| 100-node SVG | 93.0 | 87.7 | 62.0 | 56.3 |
| Native PNG | 174.0 | 170.3 | 186.1 | 182.6 |
| Final three warm Dagre SVGs | 12.6–22.2 | 10.9–20.1 | 12.5–22.0 | 11.2–16.7 |

Both 13-case runs passed, including HTTP 422 for malformed source and successful
rendering afterward. The separate `/do` calls within the Worker run also passed.
Timing is a bounded feasibility sample, not a throughput benchmark.

- Wasm: **56,053,783 bytes**, SHA-256
  `e1e59f2ed873d1cae152f7434234b303364934902cca2417efef04ebe548196a`.
- Wrangler bundle: **54,761.53 KiB uncompressed** (about 53.48 MiB),
  14,212.03 KiB gzip. Wasm and JavaScript are both included.
- Initial Wasm linear memory: 24.75 MiB; after the first render: 39 MiB.
  Highest observed linear memory: **45.5 MiB** after the final object render.
- Largest post-request V8 JS heap used: about **2.55 MiB**, separately measured
  by `Runtime.getHeapUsage`. Go heap statistics are also in the JSON; the Go heap
  is inside Wasm memory and must not be added to it again.
- Local process readiness: 990–1297 ms, including local tool/runtime startup.
  This is not Worker global-scope startup time.
- [Wrangler startup log](results/startup.log): 22.3 ms profile window, only one
  1.6 ms idle sample and zero captured active samples. Sampling missed the small
  JavaScript initialization here; **zero samples is not zero startup cost**.
  Go initialization is lazy and appears in the cold request measurements.

Request wall time is measured by the Node harness outside the Worker. Active
time is the workerd inspector's V8 CPU sampling profile, summing non-idle samples
(including GC), not Node CPU or Cloudflare billing telemetry. Inspector overhead,
sampling, JIT compilation, and this machine's CPU affect these numbers. Linear
memory comes from `instance.exports.mem.buffer.byteLength` and cannot shrink,
so it captures growth during the tested sequence. Post-request JS heap samples
are not a measurement of peak total isolate memory or compiled native code.

## Hosting conclusion and limits

As checked on 2026-10-10, Cloudflare documents a **64 MiB uncompressed bundle**
limit, **128 MB per isolate** including JavaScript and Wasm, **one second** of
global-scope startup, and **10 ms CPU per free Worker HTTP request**. This bundle
fits the documented size ceiling and these small cases show no linear-memory
blocker. The full 1000-object server allowance, concurrent calls, long-lived
memory behavior, and deployed startup admission remain untested.
[Workers limits](https://developers.cloudflare.com/workers/platform/limits/).

Durable Objects have a 30-second default CPU budget per invocation and are
available on the free plan with SQLite-backed classes. The measured compute
costs make this a plausible host, including without persistent writes, subject
to that plan's separate quotas and deployed validation. A paid Worker is another
plausible host. This is an inference from the local measurements and documented
budgets, not a deployment or cost guarantee.
[Durable Object limits](https://developers.cloudflare.com/durable-objects/platform/limits/).

Local configured limits are not enforced like deployed limits. An authoritative
cloud startup/CPU/memory admission result requires an authorized deployment,
which is outside this task. No environment or network permission expansion was
needed for the local test.
[Wrangler limits](https://developers.cloudflare.com/workers/wrangler/configuration/#limits),
[local startup profiling](https://developers.cloudflare.com/changelog/post/2026-07-31-wrangler-startup-profile-summary/),
[precompiled Wasm support](https://developers.cloudflare.com/workers/runtime-apis/webassembly/).

## Material PR #7 compatibility changes

The refresh intentionally reduces the accepted diagram/file surface. These are
real upgrade considerations, not transparent implementation details:

| Existing usage | Refresh behavior and migration |
| --- | --- |
| Remote/local/data-URL icons or images | Rejected. Use native shapes or remove the assets. There is no opt-in asset allowlist yet; asset-dependent diagrams need a separately scoped feature before adopting this branch. D2 also omits Markdown images. |
| Layers, scenarios, or steps | Rejected as multi-board input. Export/submit each board as a self-contained source; flatten inherited/shared definitions as necessary. Automated board extraction is not provided. |
| Absolute or ambient `file_path` | Start with `--input-dir /approved/root` and send a relative `.d2` path. Imports must stay inside the root. Inline `code` is suitable for agents without server-local files. |
| Writing beside an input or overwriting an artifact | Configure `--output-dir`; use an unused `output_name`, or `--write-files` for unique filenames. Results also remain inline. |
| Supplying both `code` and `file_path` | Choose exactly one; both now return an error. |
| Assuming SVG is image content | Handle the returned embedded text resource with MIME `image/svg+xml`. PNG remains standard MCP image content. |
| Relying on the old purple/sketch appearance | Pass Dagre, `theme_id: 6`, `sketch: true`, `padding: 5`. Native PNG rasterization can still differ. |

The optional remote-document interface in PR #7 is disabled by default and has
no backend. Neither this compute probe nor stateless HTTP makes documents
persistent. See the repository's [migration notes](../../README.md#compatibility-with-12).
PR #7 remains at its independently reviewed, green-CI head; this experiment is
on a separate branch and changes no production files.
