# Run on Cloudflare

This target runs the same Go MCP server in Wasm inside a SQLite-backed Durable
Object. The object supplies the rendering CPU budget; it stores no documents,
artifacts or sessions. Only `/mcp` is exposed, with a required bearer token.

## Build and run locally

Requires Go 1.27.2+, Node 24+ and npm:

```sh
cd cloudflare
npm ci --ignore-scripts
npm run build
npm test
```

For an interactive local server, put `D2_MCP_TOKEN=local-only-test-token` in
`cloudflare/.dev.vars` (ignored by Git), then run `npm run dev`. Connect to
`http://localhost:8787/mcp` with `Authorization: Bearer local-only-test-token`.
That fixture token is for local testing only.

## Deploy

Use an existing Cloudflare account on the **Workers Free** plan and confirm its
account/subdomain before publishing:

```sh
npx wrangler whoami
npx wrangler deploy --dry-run --outdir build/deploy
# After approving the account and public endpoint:
npm run deploy
npx wrangler secret put D2_MCP_TOKEN
```

Supply your approved client token at the secret prompt. Missing tokens return
503; incorrect tokens return 401. Configure the same bearer token in your MCP
client. Never commit it or put it in the URL.

The configured Worker name is `d2-mcp`; its endpoint will be
`https://d2-mcp.<ACCOUNT_SUBDOMAIN>.workers.dev/mcp`. Deployment creates that
public HTTPS route and one SQLite-backed Durable Object namespace. No custom
domain, storage backend, paid plan or billing enrollment is needed by the config.
Confirm the actual account is Free: a Paid account has different billing rules.

[Free Durable Object quotas](https://developers.cloudflare.com/durable-objects/platform/pricing/)
fail requests after the daily allowance rather than adding Free-plan charges.
Cloudflare still enforces its [runtime limits](https://developers.cloudflare.com/durable-objects/platform/limits/).
Local tests and the dry-run bundle check pass; live deployment/account admission
remain unverified because the available Wrangler session is unauthenticated.

The deployment accepts complete `code` and native render options. Local files,
image assets, document storage and the optional UI are disabled. Cancellation
is cooperative; the platform supplies the outer execution limit. Local tests
cover aborted requests; live client-disconnect propagation remains unverified.
