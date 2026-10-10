# Compatibility with 1.2.0

This comparison was checked against the actual `1.2.0` tag,
`54cdaab712a1fa598e9901192895ac213b5e1771`, using real stdio tool calls.
Images, boards, and file pipelines use the bounded behavior below. No incremental
editor or storage service is added.

| Workflow | Verified 1.2.0 behavior | Refreshed behavior and migration |
| --- | --- | --- |
| Simple source input | `code` or unrestricted server-local `file_path`; code took precedence if both were supplied | Exactly one source. Inline `code` needs no files; `--input-dir` enables relative `.d2` paths and contained imports. |
| Multi-board source | Accepted; SVG returned only the root. A child marker did not appear in output. There was no selector or batch export. | Root remains the default. `board_path: ["layers", "overview"]` selects a board; nested pairs and inherited scenarios/steps work in SVG, PNG, and ASCII. Max 32 boards, with aggregate object/edge limits. |
| Local icons and image shapes | SVG retained the local pathname; it did not embed file contents. Display depended on the consuming viewer's access. | `--asset-dir ./assets` allows relative PNG/JPEG/GIF/SVG files inside that root. Returned SVG/PNG embeds the decoded image, making the artifact independent of the original pathname. |
| Inline image data URLs | SVG preserved a quoted data URL. | `--allow-inline-assets` permits bounded base64 PNG/JPEG/GIF/SVG, including agents without local files. Quote the whole URL in D2. |
| Remote icons | SVG retained the URL. The local fixture recorded zero server requests; a viewer could fetch the URL later. | Network image URLs are rejected, even with asset opt-ins. Download an approved image to the asset root or provide bounded inline data. Neither server nor returned SVG needs an image network fetch. |
| SVG/GIF asset fidelity | Passed image references through to the consuming viewer | Supported SVG is rasterized through D2's strict, network-free importer; raw SVG markup is never embedded. Unsupported text/filter/mask or external-reference features fail explicitly. GIF uses its first frame. Use native shapes or pre-rasterize an unsupported asset. |
| File output | `--write-files` wrote next to input and overwrote that same file on the second call | `--output-dir` controls the destination. Default writes are exclusive. Use explicit `output_name` plus `overwrite: true` and server `--allow-overwrite` for a stable filename. Automatic `--write-files` names remain unique. |
| Existing links and concurrent output | `os.WriteFile` could truncate an existing path | All writes stage privately. Exclusive publication needs filesystem hard-link support and fails safely if unavailable. Opt-in replacement renames the complete file's directory entry. Existing symlinks/directories are refused; a hard-linked file is replaced without changing other links. Concurrent replacement is last-completed-writer wins, not CAS. Atomicity follows host `os.Rename` semantics. |
| SVG MCP result | Text plus base64 `image/svg+xml` image content | Embedded text resource with original SVG XML. PNG remains image content, ASCII remains text. File output returns the server-local artifact path/link; omit it for an inline result. |
| Appearance | Dagre, GrapeSoda, sketch, padding 5 | D2 defaults. Restore settings with `layout: "dagre"`, `theme_id: 6`, `sketch: true`, `padding: 5`; native PNG rasterization may differ. |

Compilation now admits a deliberately bounded subset of upstream D2. Source must
be UTF-8 without NUL bytes. Combined map/dotted-key depth and import-chain depth
are capped at 16. Admission estimates at most 10,000 AST operations and 256 KiB of
expanded scalar data across imports, classes, style rules, and boards; repeated
import occurrences also count toward a 256 KiB expanded-source budget. Imports
are frozen before either compiler pass. These conservative checks may reject
diagrams below the graph object limit.

Literal scalar variables, attribute-only classes, and attribute-only wildcard
rules remain supported. Map/array or recursively substituted variables,
structural classes, wildcard creation of objects/connections, and `${...}`
interpolation in block strings are unsupported. Expand those constructs in the
client and send explicit source. Indexed wildcard styling of existing edges,
ordinary scalar interpolation, `vars.d2-config`, and `vars.d2-legend` remain
available. Mathematical labels are limited to 4096 bytes; ordinary mathematics
render in both SVG and native PNG. SVG rejects unsupported CSS functions,
escapes, comments, and external resource references.

HTTP POSTs accept one JSON-RPC object, including on legacy SSE; batches now fail
with HTTP 400. Current-protocol disconnects cancel work cooperatively. Older
protocol calls retain their application deadline because of SDK behavior.
Checks between stages and 64 KiB write chunks stop further work after cancellation;
slots remain occupied until synchronous work actually exits. Temporary-file
cleanup never removes the final filename. Cancellation cannot undo a completed
publication or backend commit; a successful committed result is retained when
available. Individual compiler, decoder, and filesystem calls may still run to
completion before the next check.

For a trusted local project:

```sh
mkdir -p diagrams assets artifacts
d2-mcp --input-dir ./diagrams --asset-dir ./assets \
  --output-dir ./artifacts --allow-overwrite
```

Then call `render-d2` with:

```json
{
  "file_path": "architecture.d2",
  "board_path": ["scenarios", "failure"],
  "format": "svg",
  "output_name": "architecture.svg",
  "overwrite": true
}
```

The source can contain `service: {icon: "icons/service.svg"}` when that file is
under `assets/icons/`. Asset paths are relative to the asset root, not to the
input file's directory. The operator chooses these roots; remote callers cannot
change them in a tool request. Defaults expose no asset root and no overwrite
capability. Enabling these flags for a remote server grants its authorized users
access to the selected assets/output directory, so choose project-specific roots.

Each board is a standalone artifact. No HTML viewer, multi-file export, animation,
or cross-board navigation is implied. Validation also checks unselected boards.
Asset limits and unsupported SVG constructs produce explicit tool errors. The
optional document-store adapter still has no shipped backend; stored source must
be portable and cannot refer to server-local files or assets.

## Evidence and limits of the comparison

The baseline Go server was built without source edits. Its `oss.terrastruct.com/d2`
vanity module download was forbidden by the available route, so a temporary
`-modfile` replaced only that dependency with the official GitHub `v0.7.1` source,
commit `6408af31d`. Go 1.27.2 compiled the old server's Go 1.25.5 module. The
remaining dependency versions came from its original module file.

Real `initialize` and `tools/call` exchanges verified root-only board output,
local/remote/inline SVG references, absolute file input, and two writes to the
same sibling SVG. The old D2 logger printed non-JSON warnings on stdout; the
probe skipped those lines to inspect tool results. The refreshed server already
fixes that stdio problem. Baseline PNG was unavailable because `rsvg-convert` was
not installed, so old PNG asset rendering is **not** claimed as tested.

Current tests decode actual native PNG pixels for rooted PNG/JPEG/GIF/SVG and
inline PNG/SVG, check embedded SVG output, source/asset path escapes and network
silence, reject active/external SVG, bound repeated-asset expansion before SVG
allocation, verify root/nested/inherited board selection, and exercise file
replacement over both handlers and real stateless HTTP. These are regression
checks, not a security audit or process-isolation guarantee.
