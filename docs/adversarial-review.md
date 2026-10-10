# Refresh review, 2026-10-10

Independent reviewers examined the refresh at
`50454d4e60aec1524f07aefb3f811ced196f69e6`, covering rendering/compiler limits,
files and emitted artifacts, MCP transports and document isolation, and client
installation. The following findings were reproduced, fixed, and checked again
by reviewers other than the fix author. This is a focused engineering review,
not a security audit. The PR records the final commit and its CI result.

| Priority | Confirmed issue | Change and evidence |
| --- | --- | --- |
| P1 | Small nested or structurally expanded D2 could consume excessive resources before post-compilation graph checks; compilation ignored cancellation. | Pre-compilation admission bounds depth and projected expansion, restricts structural templates, and freezes admitted imports. A 200-level input previously exceeded a bounded three-second probe; the same input was rejected in about 2 ms after the change. Wildcard connection expansion was rejected in about 5 ms. |
| P2 | LaTeX-contributed SVG CSS could retain external resource references for the viewer to fetch. | Emitted CSS admits a conservative subset and strict local fragments. URL casing, escapes, alternative image functions, custom properties, and quote-layer variants fail. Browser probes intercepted and aborted requests; no external test fetch was allowed. |
| P2 | Legacy HTTP JSON-RPC batches multiplied small request bodies into large buffered results. | Every HTTP POST accepts one JSON object. A 216 KiB, 2048-call batch previously produced about 11.85 MB; it now returns HTTP 400 with a short error, including through the SSE message endpoint. |
| P2 | Current-protocol HTTP disconnects did not cancel backend work or release its operation slot promptly. | Enable the SDK's request cancellation propagation. Independent checks observed cancellation below 1 ms, isolated unrelated concurrent principals, and verified slot reuse. Older protocols retain the configured application deadline. |
| P2 | Ordinary mathematical labels failed in native PNG because SVG import budgets were absent. | Configure finite SVG import limits and cumulative budgets. Tests decode actual PNGs and find colored formula pixels in both node and edge labels. |
| P2 | Legend size was omitted from pre-render dimension checks. | Include conservative legend text/layout metrics. Oversized legends fail validation and rendering consistently; ordinary mixed legends render successfully. |
| P2 | Negative explicit dimensions passed validation/SVG but failed native PNG. | Reject negative/non-finite width and height before layout; retain bounded signed positions. |
| P3 | Windows drive paths produced an incorrect artifact file URI. | Serialize drive, UNC, and extended native paths correctly. Unit tests cover their URI representation; Windows runtime file operations were not exercised locally. |
| P3 | Generated Codex TOML retained literal DEL characters and could be invalid. | Escape DEL while preserving non-BMP Unicode. Parser round trips cover controls, quotes, Unicode paths, and arguments. |
| P3 | Endpoint validation accepted malformed hostnames that clients rejected or interpreted differently. | Require conservative ASCII/punycode DNS or canonical IP addresses. Tests cover invalid hosts, supported Unicode URL paths, loopback, and IPv6. |

## Regression coverage

Committed regressions live in `adversarial_test.go`, `d2/preflight_test.go`, and
`agents/test_build.py`. They supplement real rendering, rooted file/asset and
replacement, board inheritance, stdio, SSE, stateless HTTP, protocol negotiation,
and optional document-store concurrency tests. Normal Markdown, code, tables,
classes, mathematical expressions, gradients, dark themes, animated edge styling,
legends, and imported diagrams were checked for compatibility. The
[compatibility notes](compatibility.md) explain the deliberately narrower source
subset and format/transport changes.

## Remaining limits

Admission checks are estimates, not preemptive CPU/memory isolation. D2 parsing,
compilation, and some rendering work are cooperative or non-interruptible; an
operator handling untrusted workloads still needs process resource limits.
Authentication and authorization of a custom document backend remain the
integrator's responsibility. The shipped server has no backend or public host.

The formal security-scan workflow could not run because its required preflight
resource was unavailable. An additional LaTeX macro stress-test pass was blocked
by the platform safety filter and was not retried; this review does not claim
coverage of that pass. Ordinary mathematics, source-size limits, importer budgets,
and the previously reproduced SVG variants were checked.

The old release's PNG renderer could not be compared without `rsvg-convert`.
Local multistage Docker dependency fetching was blocked by container DNS; CI
covers the full image build and non-root MCP calls. Native client checks covered
Claude Code and Pi. Codex configuration parsing was exercised, but full plugin
installation into its normal state directory was outside the writable workspace.
No permissions, credentials, merge, release, deployment, or separate Cloudflare
experiment were changed by this review.
