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

[module-testing.md](module-testing.md) is the full procedure for a module; [patterns.md](patterns.md) lists what has been found.

## Setup

Set the variables once, as in the README: `WEBSCOPE_DB_URL` (required), `WEBSCOPE_APP_URL` (`page`, exact origin of the tab), `WEBSCOPE_BACKEND` and `WEBSCOPE_FRONT` (`code`), `WEBSCOPE_TENANT_QUERY` (a tenant lookup other than Hyn Tenancy's). A missing variable fails with `<NAME> is not set` and does not guess; `fields` without `WEBSCOPE_BACKEND` still answers, but without readers and writers.
