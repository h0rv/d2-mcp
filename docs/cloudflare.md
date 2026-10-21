# Run on Cloudflare

This optional deployment target, included in 2.0.0, runs the same Go MCP server
in Wasm inside a SQLite-backed Durable Object. The object supplies the rendering
CPU budget; it stores no documents,
artifacts or sessions. `/` serves a small static landing page; `/mcp` serves the
protocol. MCP access is private by default; public, token-free access requires an
explicit runtime setting. The landing page describes the public `d2.horv.co`
service; customize its endpoint and access copy for a private deployment.

## Build and run locally

Requires Go 1.27.2+, Node 24+, npm and Bash (use WSL or Git Bash on Windows):

```sh
cd cloudflare
npm ci --ignore-scripts
npm run build
npm test
```

For an interactive local server, put `D2_MCP_TOKEN=local-only-test-token` in
`cloudflare/.dev.vars` (ignored by Git), then run `npm run dev`. Connect to
`http://localhost:8787/mcp` with `Authorization: Bearer local-only-test-token`.
That fixture token is for local testing only. To test public access instead, set
`D2_MCP_PUBLIC=true` in `.dev.vars` and omit `D2_MCP_TOKEN`.

## Deploy

Use an existing Cloudflare account on the **Workers Free** plan and confirm its
account/subdomain before publishing:

```sh
npx wrangler whoami
npx wrangler deploy --dry-run --outdir build/deploy
# After approving the account and public endpoint:
npm run deploy
```

### Choose access

For **public access**, after deploying this version, open **Worker → Settings →
Variables and Secrets → Add**. Add a **Plaintext** runtime variable named
`D2_MCP_PUBLIC` with the exact value `true`, then select **Deploy**. Leave
`D2_MCP_TOKEN` unset. Clients connect without an Authorization header. This is a
runtime variable, not a Workers Builds variable. The config uses `keep_vars: true`
so later GitHub deployments preserve the dashboard choice.

For **private access**, leave `D2_MCP_PUBLIC` unset or set it to `false`, and add
`D2_MCP_TOKEN` as a runtime **Secret** in the same dashboard section (or run
`npx wrangler secret put D2_MCP_TOKEN`). Clients send `Authorization: Bearer <token>`.
Never commit the token or put it in the URL.

Neither mode configured returns 503. Invalid flag values, or `true` together with
a token, also return 503; incorrect private bearer tokens return 401.

### Public request limits

The included Cloudflare Rate Limiting binding admits **60 requests per 60 seconds
per Cloudflare location**, shared by all public callers to `/mcp`. Discovery,
invalid requests and rendering calls all count. The check runs before forwarding
to the rendering object. Excess requests return **429** with `Retry-After: 60`;
a missing or failing limiter returns 503. Private bearer requests do not use this
public allowance.

[Cloudflare's limiter](https://developers.cloudflare.com/workers/runtime-apis/bindings/rate-limit/)
is approximate and eventually consistent, not a hard global request or spending
cap. A busy caller can consume the shared allowance, and traffic across locations
has separate allowances. Keep namespace ID `20261015` unique to this limiter in
your account; change it in `wrangler.jsonc` if already used.

Existing Go limits still apply: 2 MiB request bodies, 256 KiB D2 source, two
simultaneous diagram operations, 16 MiB artifacts and a cooperative 30-second
operation timeout. The single rendering object retains Cloudflare's default
30-second CPU limit per invocation. No documents, sessions or per-user counters
are stored by the application. Public access can exhaust Free-plan quotas;
rate limiting does not prevent charges on Paid accounts.

### Endpoint and platform

The canonical endpoint is `https://d2.horv.co/mcp`. Wrangler declares
`d2.horv.co` as a custom domain for `d2-mcp`, so GitHub Builds deployments retain
the route. The `workers_dev` setting remains enabled, preserving
`https://d2-mcp.<ACCOUNT_SUBDOMAIN>.workers.dev/mcp` as an alternate endpoint.

For your own deployment, replace or remove the custom-domain route in
`cloudflare/wrangler.jsonc`; a custom domain requires a zone in your Cloudflare
account. Deployment also creates one SQLite-backed Durable Object namespace.
No storage backend, paid plan or billing enrollment is needed by the config.
Confirm the actual account is Free: a Paid account has different billing rules.

[Free Durable Object quotas](https://developers.cloudflare.com/durable-objects/platform/pricing/)
fail requests after the daily allowance rather than adding Free-plan charges.
Cloudflare still enforces its [runtime limits](https://developers.cloudflare.com/durable-objects/platform/limits/).
Local tests cover a standalone Node-hosted Go Wasm render and the hosted MCP
behavior; the dry-run bundle check validates the deployment configuration.

The deployment accepts complete `code` and native render options. Local files,
image assets, document storage and the optional UI are disabled. Cancellation
is cooperative; the platform supplies the outer execution limit. Local tests
cover aborted requests; live client-disconnect propagation remains unverified.

## Code boundary

`main_wasm.go` is the generic js/wasm HTTP bridge, paired with the native entry
point. It has no Cloudflare imports, credentials or infrastructure configuration.
The shared Go application registers the tools, applies limits and renders D2.
Only `cloudflare/` knows about Worker routing, bearer authentication and Durable
Objects. This optional target needs JavaScript and Wrangler; the native core does
not. The short Bash build command only invokes Go and copies its matching Wasm
runtime and license. No Python, document storage or provider-specific Go server
is introduced.

The landing page lives in `cloudflare/site/`. Only its HTML and copy script are
bundled as text modules. GET/HEAD requests for `/` and `/copy.js` do not call the
renderer or the MCP limiter; other methods return 405. Unknown routes, including
the removed `/example.*` showcase routes, return 404. No deployment bindings or
MCP behavior change with this page.

The page leads with remote client setup and a copyable agent prompt. Client tabs
support keyboard selection and fall back to visible instructions without
JavaScript. Copy controls select the text if clipboard access is unavailable.
Browser checks reuse the existing `apps/` test dependencies; the page itself has
no framework, dependencies or external assets. Client documentation links are
listed in [agent installation](agent-installation.md#remote-http).

The former diagram assets remain in `cloudflare/testdata/` as integration
fixtures, outside the Worker bundle. Worker tests still compare SVG, PNG and
ASCII bytes with fresh MCP renders, using `padding: 24` and otherwise default
options. From `cloudflare/`, run `npm run build && node render-fixtures.mjs` to
regenerate them. Removing the showcase does not remove ASCII rendering coverage.

See the [ASCII diagnosis](ascii-rendering.md) for the former showcase’s spacing
problem and the separately reproduced upstream Dagre arrowhead defect.
