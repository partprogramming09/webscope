# AGENTS.md

webscope is an MCP server (Go, stdio) with four tools for testing a web application with few tokens: `query`, `page`, `check` and `code`. Build with `go build -o webscope .`; test with `go vet ./... && go test -race -count=1 ./...` (the `page` tests open Chrome; `WEBSCOPE_CHROME` points to one).

## Using the tools

- Start with `code fields` and `query` (the table), then `code map` or `code map_back` (the endpoints and tables a screen touches), `code snap` (keep the table), `page` (`load`, `state`, `fill`, `click`), `check` (sent vs saved vs read), `code snap_diff` (what the action wrote).
- `page(help)` lists the actions; `code(help)` lists the code actions. For a migration, call `code(help, {topic:"migration"})` first.
- The contract of every tool and how to report a test: `docs/usage.md`.

## Rules that hold

- The server never writes to the database. Test writes go through the screen, on rows prefixed `ZZ_`, and are cleaned up through the screen.
- With customer data, ask for the schema and counts, not rows.
- A failure reads `Failed: <what happened>. Next: <what to do>.` Report it as it is.
- Report a test with what was verified (a real action, an observed effect) apart from what was not. A download counts only if the file is in the downloads folder.

## Changing the code

- `main.go` only registers the tools; the rest is in `internal/`. Tests sit next to their code.
- At most 4 tools and a `tools/list` of 2500 bytes or less (the test in `conformance_test.go` uses 1800); every version measures it again.
- Each version is one commit: the code, `version` in `main.go`, the same version in `.claude-plugin/plugin.json` (a test compares them), the status line of the roadmap and an entry in `CHANGELOG.md`.
- `docs/` holds the guides, the ADRs and the research; `CLAUDE.md` has the architecture.
