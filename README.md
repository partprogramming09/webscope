# webscope

[![go](https://github.com/partprogramming09/webscope/actions/workflows/go.yml/badge.svg)](https://github.com/partprogramming09/webscope/actions/workflows/go.yml)

[Español](README.es.md)

An MCP server that lets an AI agent test a web application and understand its data and code while spending very few tokens. It reads your database, looks at and drives a screen in your Chrome, checks that what a screen sent is what ended up saved, and reads the project's code. One Go binary, four tools, and it never writes to your database.

## Why it is useful

When you ask an agent to «test the quotes screen», the expensive part is not thinking: it is reading. Whole pages of HTML, hundreds of table rows, the output of a `grep` over a backend. webscope answers with short, capped, structured text instead, so the agent can do the whole round (look at the screen, fill it, save, see what was stored, see what else changed) in a handful of calls.

It works on any web app and any MySQL or SQLite database. It was built against a real Laravel + Vue application and its older PHP version, so some pieces are tied to a stack: `map`, `map_back`, `contract`, `flows` and `cases` to Laravel routes and Vue modules, and `legacy` to legacy PHP with the SQL inside the file and jQuery calls. The rest is generic.

## How it works

1. Your agent (Claude Code, or any MCP client) starts webscope as a local program and talks to it over stdio.
2. webscope answers from three places: your database (read-only), your Chrome (it hands the agent a short JavaScript snippet to run; it does not drive the browser itself) and the folders of your project's code (read-only).
3. Every answer is short and says what it left out. The agent decides what the findings mean; webscope only reports facts it can check.

## What you can ask your agent

With the server registered, ask in plain words:

- «How many rows does `quotes` have, and which columns are always empty?» (`query`, `code fields`)
- «Open the quotes screen, fill the form with test data, save it, and tell me if what the screen sent is what was stored.» (`page`, `check`)
- «What did saving that quote really write? List every row it touched, child tables included.» (`code snap`, `snap_diff`)
- «Make the quotes list answer a 500 and tell me what the screen shows.» (`page simulate`)
- «Audit this screen: text that was not rendered, controls covered by something, fields nobody can name.» (`page audit`)
- «Which endpoint, controller and table does this screen use, and does it send what the endpoint validates?» (`code map_back`, `code contract`)
- «Does the new screen send and receive the same fields as the old one?» (`page shape`, `page parity`)
- «What are the flows of this module, what should I test, and does the migration keep what the legacy did?» (`code flows`, `code cases`, and «Migrating a module» below)

## The four tools

| Tool | In one line | It needs |
|---|---|---|
| `query` | One read-only SQL statement; the answer is a short table. | `WEBSCOPE_DB_URL` |
| `page` | Read, fill and click a screen in your Chrome, see the requests it sent, force an HTTP error, audit it, and compare an old screen and a new one. | Claude in Chrome and `WEBSCOPE_APP_URL` |
| `check` | For one saved row: what the screen sent, what the database stored, what the screen read back. Only the differences. | `page` and the database |
| `code` | The sheet of a table, who uses a column, the routes and calls of a screen, what an action changed, the flows and test cases of a module, and what a legacy PHP module calls and writes. | `WEBSCOPE_BACKEND`, `WEBSCOPE_FRONT`, `WEBSCOPE_LEGACY` |

A typical session, in the order that works best:

1. `code fields` and `query`: learn the table and its current counts.
2. `code map` or `code map_back`: learn which endpoints and tables the screen touches.
3. `code snap`: keep the table as it is before the action.
4. `page`: `load`, `state`, `fill`, `click`.
5. `check`: compare the saved row with what was sent and read back.
6. `code snap_diff`: see every row the action wrote.
7. Clean up through the screen, never through the database.

## Getting started

### Requirements

- An MCP client. The steps below use Claude Code; [docs/install.md](docs/install.md) covers Codex, Cursor, VS Code and Gemini CLI.
- Go 1.27.3 or newer, to build the server (`go.mod`).
- For `query`, `check` and `code fields`: a MySQL or SQLite database.
- For `page` and `check`: Google Chrome with the Claude in Chrome extension and a tab open on the app you test.
- For `code`: the source folders of the app (a Laravel backend and a Vue front for `map`, `map_back`, `flows` and `cases`).

### Install as a Claude Code plugin

    claude plugin marketplace add partprogramming09/webscope
    claude plugin install webscope@webscope

Then `/plugin configure webscope@webscope` asks for the database URL (stored as a sensitive value) and the other settings; an empty one leaves its tool off. Do not pass the URL on the command line: it stays in the shell history and shows in `ps`. The plugin brings the server and four skills. The first start builds the binary (8 to 14 s depending on whether Go's module cache is warm; `bin/webscope-launch build` does it ahead of time).

### Or build it yourself

    go build -o webscope .
    claude mcp add webscope \
      -e WEBSCOPE_DB_URL='mysql://user:pass@127.0.0.1:3306/dbname' \
      -e WEBSCOPE_APP_URL='http://localhost:5173' \
      -e WEBSCOPE_BACKEND='/path/to/backend' \
      -e WEBSCOPE_FRONT='/path/to/front' \
      -- /absolute/path/to/webscope

Only `WEBSCOPE_DB_URL` is needed to start (`query`); each other variable switches on one more tool. All of them, with what each does, are in [docs/reference.md](docs/reference.md#environment-variables).

### Check that it works

Reconnect with `/mcp` (a connected client keeps talking to the old process), then ask the agent for `query` with `SELECT 1` and for `page(help)`.

## What it guarantees

- **It never writes to the database.** `query` is read-only twice (a statement check, and a read-only session); `check`, `code` and `snap` only read.
- **Writes happen only through the screen**, with your approval, on rows prefixed `ZZ_`. Cleaning up is done through the screen too.
- **Answers are small and say what they left out**: 50 rows (at most 500), 200 characters per cell, 8000 bytes per answer. The list of tools the agent has to load stays under 1800 bytes (a test checks it).
- **`page` is local**: a server on `127.0.0.1` with a random port and token, which answers only the origin of your app.
- **No guessing**: `check` takes the field-to-column map from you, and every answer of `code` that comes from searching text says it is an estimate.
- With customer data, ask for the schema and for counts, not for rows.
- **The tool reports, the model decides**: webscope only labels facts it can check and never blocks an action. How to report a test honestly is in [docs/usage.md](docs/usage.md#reporting-a-test).

## Migrating a module from a legacy system

The case this was extended for: a module that exists in an old system and was rebuilt, and you want to know what the new one lacks, adds or changes. It takes two origins (`WEBSCOPE_APP_URL=http://old:8088,http://new:5173`), the legacy folder (`WEBSCOPE_LEGACY`) and two tabs of Chrome with the session started. Everything below reads; the only writes are one test row per system through the screen, prefixed `ZZ_`, and only with your approval.

| Step | Ask the agent | What it runs | What comes back |
|---|---|---|---|
| 1. What the module does | «Give me the flows and user stories of `crm/quotes`» | `code flows` | Screens and actions with their conditions and permissions, then endpoints, controllers and tables, as evidence; the stories are written from it and you confirm them |
| 2. What the old code touches | «What does the legacy quotes module call and write, and which tables does the new one not have?» | `code legacy {path, module}` | Each legacy endpoint with its caller and tables; the tables only one side has, split into «named elsewhere in the backend» and «not named» |
| 3. Screens | «Compare the quotes list of the old and the new screen» | `page shape {name, selector}` on each, `page parity {a, b}` | Only the fields, buttons and columns one side has |
| 4. Requests | «Does the new search send and receive what the old one did?» | the same action on both, `page parity {req_a, req_b, map}` | Status and the field names only one side sends or gets, never values |
| 5. What to test | «List the test cases for this module» | `code cases` | A table: happy path, permission, required field, status, HTTP error and empty list, with what runs each |
| 6. A write on both | «Create a `ZZ_` quote on each system and compare what was saved» | `code snap`, `page fill` and `click`, `code snap_diff`, `code rows`, `query` | The rows each system wrote, their differences, and the child rows by count |
| 7. Unit tests | «Plan the unit tests that pin the old behaviour» | `code help {topic: "migration"}`, step 7 | How to write each confirmed difference as a unit test, in the layer that owns the rule |

The whole chain, step by step, is `code help {topic: "migration"}`; the procedure for a whole module is [docs/module-testing.md](docs/module-testing.md). A real run on a quotes module found, among other things, that 21 of 22 tables «only in the legacy» were in fact used by the new backend and that the old screen was saving a number the new one had already taken (`Duplicate entry … 1062`). `CHANGELOG.md` has the detail of each run and what was not checked.

## Getting help

- **A question or a bug:** open an [issue](https://github.com/partprogramming09/webscope/issues). Say the version (`webscope` prints it on start), what you asked the agent, what you expected and what happened; a failure reads `Failed: <what happened>. Next: <what to do>`, so paste that line.
- **A vulnerability:** do not open an issue; follow [SECURITY.md](SECURITY.md).
- **How to use it:** [docs/usage.md](docs/usage.md) is the contract of each tool and the order to use them in; [docs/patterns.md](docs/patterns.md) lists the findings met while testing screens and how to recognize each.

## Documentation

| Document | What it has |
|---|---|
| [docs/usage.md](docs/usage.md) | The contract of each tool, and how to use the four together. |
| [docs/reference.md](docs/reference.md) | Every variable and argument, what was measured on a real application, known limits and what is not in this version. |
| [docs/install.md](docs/install.md) | Installing in Claude Code (plugin or not) and in other agents. |
| [docs/module-testing.md](docs/module-testing.md) | The procedure to test a module as a user and as an attacker. |
| [CHANGELOG.md](CHANGELOG.md) | One entry per version: what changed, what was seen, how it was checked, what was not. |

## Contributing

Maintained by [partprogramming09](https://github.com/partprogramming09). Changes are welcome: [CONTRIBUTING.md](CONTRIBUTING.md) says how to build, test and change the server (`go vet ./...` and `go test -race -count=1 ./...` are what CI runs on Linux and macOS). Each version is one commit with its changelog entry.

## License

[MIT](LICENSE).
