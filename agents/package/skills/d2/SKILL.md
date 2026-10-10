---
name: d2
description: Create, validate, render, or revise D2 diagrams using d2-mcp. Use for architecture, flow, dependency, sequence, and data diagrams when D2 source or a PNG, SVG, or ASCII artifact is wanted.
---

# D2 diagrams

Use the connected d2-mcp tools. If they are unavailable, explain that the MCP connection must be installed; do not claim a diagram was rendered. Discover `compile-d2`, `render-d2`, and `fetch_d2_cheat_sheet`; clients may add a prefix or normalize hyphens to underscores.

Inspect the advertised tool schemas before selecting options. This workflow targets the refreshed server; older releases may lack layout choices, structured diagnostics, or native PNG. If a requested feature is unavailable, explain the version limitation and use only options the connected server advertises.

1. Identify the audience, relationships, and requested artifact. Infer reasonable layout choices from the task; ask only for missing facts that change the diagram.
2. Read `fetch_d2_cheat_sheet` when syntax or supported features are uncertain. Write the complete D2 source. Keep labels short, group related nodes, and make arrow direction meaningful. Start with `dagre`; try `elk` for dense hierarchies or `tala` when its layout improves readability.
3. Call `compile-d2` with `code` and the intended layout/options. Read `isError` and `structuredContent.error`, including one-based diagnostic locations. Fix the source and validate again. Do not retry unchanged invalid input or silently remove requested content.
4. Call `render-d2` with the same complete source/options. Prefer PNG for viewing, SVG for a scalable downloadable artifact, and ASCII for text-only output. Inspect the returned artifact when the client permits; report rendering success separately from visual inspection.
5. Return the artifact and complete source, or save source to a user-requested local file when your client has file tools. Explain material assumptions and any limitation that prevented the requested result. For revisions, resubmit the full source and options.

## Inputs, files, and state

- `code` works with local or remote servers and agents without files. `file_path` means a relative `.d2` file on the **server**, within its configured input root; it does not upload a client file. Use exactly one input. Resolve imports into self-contained source for a remote server unless its operator has configured the required files.
- Each HTTP request is independent. Preserve the source and rendering options in the conversation or user-controlled files. The default server has no document store. Use `document_id` and `revision` only if a configured server advertises a document backend; handles work only within that backend and its authorization namespace.
- PNG is MCP image content, SVG is an embedded resource containing XML, and ASCII is text. A `urn:sha256:...` identifies returned content; it is not a download URL. A returned `file:` path is server-local. Do not invent a hosted artifact link.
- Use `output_name` only when the user wants a server-local artifact and the server has an output root. Overwrite needs both server opt-in and explicit request intent. Saving returned bytes with client file tools does not require server output configuration.
- Assets require server opt-in. Never fetch arbitrary network images or bypass configured file roots. If an asset is unavailable, describe the limitation and offer a text/native-shape alternative without silently changing the requested diagram.
- Multi-board source needs an explicit `board_path` when selecting a nested board; inspect the tool schema for its path representation. One render returns one artifact, not a board collection.

## Useful starting prompts

- “Diagram browser → API → database, with a queue and worker for background jobs. Label synchronous and asynchronous connections, validate it, and return PNG plus complete D2 source.”
- “Revise this complete D2 source to add a cache. Preserve existing relationships, validate it, and return SVG plus the updated source.”
- “Validate this D2 source. Explain located errors and show the smallest correction; do not render or write files.”

Do not deploy a server, change client permissions, enable asset/file access, or provision storage merely to complete a diagram.
