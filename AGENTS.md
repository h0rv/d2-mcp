# Repository Guidelines

## Project Structure & Module Organization
The Go entry point lives in `main.go`, which wires CLI and SSE flags and registers tool handlers from `handlers.go`. Rendering helpers and external D2 bindings sit under `utils.go` and `d2/tools.go`, while build metadata lives in `go.mod` and `Dockerfile`. Automation and release workflows are tracked in `.github/workflows`; keep additional assets near the features that depend on them.

## Build, Test, and Development Commands
- `go build .`: compile the stdio/SSE server against Go 1.24.2.
- `go run . --sse --port 8080`: start the HTTP SSE service; add `--image-type svg` or `--image-type ascii` while iterating.
- `go run . --transport http --port 8080`: start the streamable HTTP transport for MCP clients.
- `go run . --transport stdio --image-type ascii`: default stdio transport tuned for LLM-friendly ASCII output.
- `go test ./...`: execute all Go unit tests; add `-v` locally when stabilising flaky cases.
- `docker build . -t d2-mcp`: produce the container image that matches production runners.
- `npx @modelcontextprotocol/inspector ./d2-mcp`: inspect tool contracts during MCP client debugging.

## Coding Style & Naming Conventions
Use `gofmt` (tabs, 2-space visual indent) and `goimports` before submitting patches; the repo's files follow canonical Go formatting. Name handler functions with `*Handler` (e.g., `RenderD2Handler`) and keep package-level configuration prefixed with `Global`. Go packages should stay singular and lowercase (`d2`, `server`), and new flags must use descriptive kebab-case names to match existing CLI expectations.

## Testing Guidelines
Prefer table-driven tests in `*_test.go` files co-located with the code under test, using Go's standard `testing` package. When exercising diagram rendering, inject the context and stub filesystem interactions via temporary directories to keep runs hermetic. Target full coverage for new handlers and utilities and assert error paths on D2 wrappers.

## Commit & Pull Request Guidelines
Adopt the existing Git history style: short, imperative subject lines in ASCII (e.g., `fix docker setup`). Each PR should describe the behavioural impact, link any relevant issues, and call out CLI or protocol changes for MCP clients. Include before/after screenshots when altering rendered output, and document new flags or environment variables in `README.md`. Run `go test ./...` and note the result in the PR body before requesting review.

## Environment & Tooling Notes
Rendering PNG output depends on ImageMagick (`magick` or `convert`) being available on `$PATH`; ensure CI images install it when enabling PNG tests. ASCII rendering defaults to Unicode box drawing—set `--ascii-mode standard` when targets can't display those glyphs. `MCP_TRANSPORT`/`PORT` env vars let you switch transports in containerized runners. Keep Go toolchains aligned with the `go 1.24.2` directive, updating `go.mod` and `Dockerfile` together when bumping versions. The bundled `d2/CHEATSHEET.md` powers the `fetch_d2_cheat_sheet` tool—update it alongside feature changes so downstream LLMs receive accurate guidance.
