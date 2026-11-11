# Contributing

**Reader:** people who change the server

## Build and test

    go build -o webscope .                                        # Go version from go.mod
    go vet ./...                                                  # same as CI (.github/workflows/go.yml)
    go test -race -count=1 ./...                                  # the whole suite
    go test -race -count=1 -run 'TestName' ./internal/db          # one test, in its package

The `page` tests open a real Chrome: they use `WEBSCOPE_CHROME` if set, else Google Chrome at its macOS path, else `google-chrome`, `google-chrome-stable` or `chromium` on the `PATH`. Without one they fail. After rebuilding, a connected MCP client keeps talking to the old process: reconnect it (`/mcp` in Claude Code).

## Layout

`main.go` registers the four tools and wraps each handler so a panic becomes `Failed: internal error` instead of stopping the server. Each tool is a package under `internal/`; tests live next to their code.

| Package | What it holds |
|---|---|
| `internal/db` | `query`: the read-only connection, the statement check (`sqlguard.go`, safe under MySQL and SQLite rules at once) and the tenant lookup. `check` and `code` reuse it. |
| `internal/page` | `page`: returns JavaScript snippets (`page.js`, embedded) and runs the local listener that keeps captured requests and screen shapes. Also `parity`, `download`, `verify` and the backend log reader. |
| `internal/check` | `check`: one captured request against one stored row and, optionally, the read that followed. |
| `internal/code` | `code`: text search over the backend (`fields`, `consumers`), snapshots, and the stack-bound readers (`laravel.go`, `vue.go`, `flows.go`, `cases.go`, `legacy.go`). The migration playbook of `code help` is `playbook-migration.md`, embedded. |
| `internal/fsx` | File walking that never follows symbolic links and skips `vendor`, `node_modules`, `storage`, `.git` and `.env*`. |
| `internal/input` | The shape of a call (action and arguments) and the answer size cap. |

## Design rules

- The server never writes to the database. Test writes go through the screen, on rows prefixed `ZZ_`.
- Four tools at most. `tools/list` has a byte budget that `main_test.go` checks; every version measures it again.
- `query` and `page` serve any project: they are tested with SQLite and with a page without Vuetify, and carry no names of a particular application. Stack-bound actions (`map`, `map_back`, `flows`, `cases`, `legacy`) each live in their own file.
- Errors read `Failed: <what happened>. Next: <what to do>.`
- The tool reports, the model decides: webscope labels only facts it can check itself and never blocks an action. A new action is optional: not calling it changes nothing.
- Answers are capped and say what they left out.

## Versions

Each version `0.7.N` is one commit with the code, the `version` constant in `main.go` and a new entry at the top of `CHANGELOG.md` (it says so when what a tool prints changes). Message: `0.7.N: <what it does>`. A release is tagged `v0.7.N` on that commit.

## Security

See [SECURITY.md](SECURITY.md) to report a vulnerability privately.
