# Using webscope

The contract of each tool: when to use it, what it guarantees, what it cannot do. The README has the full argument reference; this page says how to use the four tools together.

## Guarantees that hold for every tool

- The server never writes to the database. `query` is read-only twice (statement check, read-only session); `check`, `code` and `snap` only read.
- Test writes happen only through the screen (`page`), with the user's approval, on rows prefixed `ZZ_`.
- Answers are short and capped; when something was left out, the answer says so.
- A failure is `Failed: <what happened>. Next: <what to do>.` and is marked as an error. A bug inside a tool answers `Failed: internal error` and the server keeps running.
- Against data that belongs to a customer, ask for the schema and for counts, not for rows.

## The tools

| Tool | Use it to | Never expect |
|---|---|---|
| `query` | Read rows or schema (`SELECT`, `SHOW`, `DESCRIBE`, `EXPLAIN`, `WITH`) | A write, a second statement, `PRAGMA`, `LOAD_FILE`, locks |
| `page` | Read, fill and click a screen in Chrome and see the requests it sent | To drive the browser itself: it returns a snippet that you run with `javascript_tool` |
| `check` | Compare what the screen sent, what was stored and what the screen read back | A guess of the field mapping: you give `map` |
| `code` | See who reads and writes a column, the routes and calls of a front module, and what an action wrote | A parsed answer: it searches text, so every answer is an estimate |

### `query(sql, limit?, tenant?)`

- Refine with `WHERE` or fewer columns when the answer says rows or cells were cut (50 rows, at most 500; 200 characters per cell; 8000 bytes).
- Escape a quote by doubling it (`'it''s'`); a backslash escape is refused because MySQL and SQLite read it differently.
- `tenant` is the fqdn of a tenant; the database is found in the central one. A central table is read by calling without `tenant`.

### `page(action, args?)`

Run `page(help)` first. The order of a session:

1. `page(load)` and run the snippet. Run it again after every navigation, reload or `/mcp`: the helper is lost and the port changes.
2. `state` to see the screen, then `fill`, `click` or `press`. `click` and `press` report the requests (method, path, status), new alerts, field errors and console errors, or `No effect seen`.
3. `payloads` for the requests and their answers; `simulate` to force an HTTP error on the next requests to a URL.
4. To compare an old screen and a new one: `shape {name, selector?}` on each (`selector` limits it to a part of the screen), then `parity {a, b}` for what only one side has. To compare what an action sends and receives: do the same action on both and `parity {req_a, req_b, map?}` (field names only, a GET's query names included, never values). `WEBSCOPE_APP_URL` may hold both origins, separated by commas.
5. `fill` warns when a name fits several fields (a list filter and a form field with the same label); name one by its id.

Not covered: `v-combobox`, date pickers, menus of other libraries, file uploads, pages whose Content-Security-Policy blocks the helper. If the screen shows the login, stop and ask the user for the session; never type credentials. Use one Chrome agent at a time.

### `check(sent, table, where, map, after?, at?, tenant?)`

- `sent` is a request `page` captured (`POST /api/quotes`); `where` picks one row; `map` says which column holds each sent field.
- `after` and `at` add what the screen read back (a later `GET` and the path of the object in its answer).
- The answer is counts plus one line per field that differs. `empty~`, `unread~` and `space~` are equivalent or unknown, not equal and not different.
- A body cut when captured is refused, not compared.

### `code(action, args?)`

| Action | Answers | Stack |
|---|---|---|
| `fields {table, tenant?}` | Per column: type, null, default, rows with a value, PHP files that read and write it | Any database; PHP files for the usage counts |
| `consumers {table, column}` or `{column_only}` | Each read and write of a column, `file:line` | PHP backend |
| `map {module, walkthrough?}` | Routes, HTTP calls, composables, stores; with `walkthrough`, what can be clicked | Vue modules (`src/modules`) |
| `map_back {query}` | Endpoint, controller, service and table for a front module or API address | Laravel (`app/Modules/**/Routes/*.php`) |
| `flows {module}` | Screens with their actions, then endpoints, controllers and tables: evidence for user stories | Vue + Laravel (`map` and `map_back` joined) |
| `cases {module}` | The test cases of the module as a table (happy path, permission, required field, status, HTTP error, empty list), each with what runs it; it runs nothing | Vue + Laravel (built on `flows`) |
| `legacy {path, module?}` | The PHP endpoints of a legacy folder with their callers and the tables their SQL writes and reads; with `module`, the tables against the new module's and the columns the legacy writes that the backend never names (inline `INSERT` and `UPDATE … SET` only) | Legacy PHP with SQL in the file and jQuery calls (`WEBSCOPE_LEGACY`) |
| `rows {table, a, b, children?, tenant?}` | How two rows of one table differ, column by column, without a value (`a filled, b empty`, `case only`, `same number, other text`), and how many rows of other tables point at each | Any database |
| `help {topic?}` | The list of actions and what each is for; `topic: "migration"` gives the steps of a migration with the tool of each | None |
| `snap {name, sql, key?, tenant?}` / `snap_diff {name}` | What a query returned, then only what changed | Any database; snapshots live in memory |

`code` reads at most 20000 `.php` files of 2 MB each and stops there without saying so.

## Recommended flow

To test a screen and know what it really wrote:

1. `code(fields)` and `query` to learn the table and its current counts.
2. `code(map)` or `code(map_back)` to learn which endpoints and tables the screen touches.
3. `code(snap)` the table before the action.
4. `page`: `load`, `state`, `fill`, `click`.
5. `check` the saved row against what was sent and read back.
6. `code(snap_diff)` to see every row the action wrote, child rows included.
7. Clean up through the screen, never through the database.

To migrate a module from a legacy system and keep what it did:

1. `code(flows {module})`, then write the user stories from it and have a person confirm them (skill `module-flows`).
2. `code(legacy {path, module})`: the legacy endpoints, who calls them, and the tables on one side only. A table only the legacy has and the backend names elsewhere is not a gap; one no backend file names is a question.
3. Open the old and the new screen (two tabs): `page(shape)` on the part to compare on each, `page(parity {a, b})`; then the same action on both and `page(parity {req_a, req_b, map})`.
4. `code(cases {module})`: what to test; run the ones you choose with `page`, on `ZZ_` rows, with `snap` before and `snap_diff` after, and `check` the saved row. Compare the row each system wrote with `rows {table, a, b, children}`: `a` and `b` pick each row, `children` is `{child table: column}`.
5. A confirmed difference becomes a plan of unit tests (skill `parity-tests`) for the `tester` agent; the whole chain is the skill `migration-parity`.

[module-testing.md](module-testing.md) is the full procedure for a module; [patterns.md](patterns.md) lists what has been found.

## Setup

Set the variables once, as in the README: `WEBSCOPE_DB_URL` (required), `WEBSCOPE_APP_URL` (`page`, exact origin of the tab; several origins separated by commas when the old and the new app answer on different addresses), `WEBSCOPE_BACKEND` and `WEBSCOPE_FRONT` (`code`), `WEBSCOPE_LEGACY` (`code legacy`, the folder of the legacy PHP), `WEBSCOPE_TENANT_QUERY` (a tenant lookup other than Hyn Tenancy's). A missing variable fails with `<NAME> is not set` and does not guess; `fields` without `WEBSCOPE_BACKEND` still answers, but without readers and writers.
