# ASCII showcase diagnosis

Investigated on 2026-10-16 against the page merged in PR #14 and D2 0.9.0
(`d5a51743b6d5c00f1ec6f8340003f5d5a6ba4eda`). There are two distinct problems:
our page's line spacing and an upstream arrowhead defect exposed by our default
layout. Removing the showcase fixes neither the upstream renderer nor the
server's ASCII layout default.

## Text integrity and presentation

The former `cloudflare/site/example.txt` is retained unchanged as
[`cloudflare/testdata/example.txt`](../cloudflare/testdata/example.txt). Its
396 bytes match fresh d2-mcp default ASCII output, a direct upstream library
render using Dagre, and the old HTML element's `textContent` exactly:

```text
SHA-256 bbd2e1444ceda8e30ef0f5477ce8d9ea1579563f4dffc87f246889abf3f09042
```

The page used `font-size: 12px` and `line-height: 1.3`, making rows 15.6px
high. In Chromium this leaves gaps between vertical box-drawing characters.
Changing only the line height to `1` closes those gaps. Desktop, mobile and
dark-mode checks found preserved whitespace, equal-width glyphs and no
overflow. This is our presentation defect, not escaped or missing text.

Direct access to `https://d2.horv.co/example.txt` from the review environment
failed with an Envoy CONNECT HTTP 403. The comparison therefore covers the
checked-in asset, local Worker response tests and local browser rendering;
it does not establish the current live bytes or the user's device behavior.

## Missing arrows and the layout choice

With the retained browser/API/database source, Dagre produces four route
points per connection. Upstream ASCII route simplification reduces them to two,
but the destination-arrow condition still uses the original route length:

- [Route simplification](https://github.com/d2lang/d2/blob/d5a51743b6d5c00f1ec6f8340003f5d5a6ba4eda/d2renderers/d2ascii/asciiroute/asciiroute.go#L67).
- [Original length passed to drawing and destination-arrow condition](https://github.com/d2lang/d2/blob/d5a51743b6d5c00f1ec6f8340003f5d5a6ba4eda/d2renderers/d2ascii/asciiroute/drawing.go#L67).

Both arrowheads disappear in a direct upstream library render as well as in
d2-mcp. The upstream CLI [selects ELK for ASCII when Dagre or no layout is
specified](https://github.com/d2lang/d2/blob/d5a51743b6d5c00f1ec6f8340003f5d5a6ba4eda/d2cli/main.go#L471).
d2-mcp currently defaults to Dagre for every format, exposing the upstream bug.

For this diagram, `render-d2` with `format: "ascii"`, `layout: "elk"` and
`padding: 24` restores both arrowheads. It matches direct upstream ELK library
output and the upstream CLI default exactly: 448 bytes, SHA-256
`bd9c6b0932543bd7f12a7ee836827cf67dce0f5ca5717f411a332b6776740c43`.
The unusual cylinder outline also comes directly from upstream's
[cylinder renderer](https://github.com/d2lang/d2/blob/d5a51743b6d5c00f1ec6f8340003f5d5a6ba4eda/d2renderers/d2ascii/asciishapes/cylinder.go),
including with ELK.

## Reproduction and remaining work

Use [`example.d2`](../cloudflare/testdata/example.d2) as complete `code` in
`render-d2`, first with `{ "format": "ascii", "padding": 24 }`, then add
`"layout": "elk"`. With an upstream v0.9.0 CLI, render the same source to a
`.txt` output; its default result matches the ELK call. The CLI also substitutes
ELK for explicit Dagre, so use the upstream library directly to compare Dagre.
From `cloudflare/`, `npm run build && node render-fixtures.mjs` regenerates the
default fixtures; `npm test` checks the real Wasm MCP results against them.

This landing revision deliberately retains the current renderer behavior and
its integration fixtures. A separate renderer fix should test a format-aware
ELK default and preserve explicit request/source layout precedence. Choosing
`layout: "elk"` is a workaround for this reproduced diagram, not a claim that
every upstream ASCII rendering issue is resolved.
