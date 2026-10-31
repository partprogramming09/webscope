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
2. `state` to see the screen (a first line says when it looks like a 404; the URL shows the names of its query parameters, never the values), then `fill`, `click` or `press`. `click` and `press` report the requests (method, path, status), new alerts, field errors and console errors, or `No effect seen`; if a request had not answered after about 2.5 s they say `Still pending` and the effect may be incomplete (look at `payloads` later). `payloads` opens with a `Problems:` line when any captured request was a 4xx, a 5xx or had no answer.
3. `payloads` for the requests and their answers; `simulate` to force an HTTP error on the next requests to a URL.
4. To compare an old screen and a new one: `shape {name, selector?}` on each (`selector` limits it to a part of the screen), then `parity {a, b}` for what only one side has. To compare what an action sends and receives: do the same action on both and `parity {req_a, req_b, map?}` (field names only, a GET's query names included, never values). `WEBSCOPE_APP_URL` may hold both origins, separated by commas.
5. `fill` warns when a name fits several fields (a list filter and a form field with the same label); name one by its id.
6. To prove a download: `download {arm:true}`, the real click in the page, then `download {wait:true}`. It gives the file's name, size, type by bytes and, for an xlsx, header names and row count (no values); no file in time is a failure. An export counts as downloaded only if this finds the file. `download {report:true}` lists the files the session saw appear, to say in the report what was left in the downloads folder; `code(leftovers {table, column})` counts the `ZZ_` rows a test left in a table. Both only list: deleting is the user's.

7. To build the final report: `verify {claim, evidence}` for each thing it will say, with `evidence` as `capture METHOD address`, `check METHOD address` (the last `check` of that request) or `download`; then `verify {report:true}` and write the report from that table. The server gives the state (OBSERVADO, INFERIDO if an interceptor was seen, NO VERIFICADO without a reference it can find).

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
| `leftovers {table, column, prefix?, tenant?}` | How many rows of a table have a column that starts with the test prefix (`ZZ_` by default): what a test left behind, a count and no values | Any database |
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

## Reporting a test

webscope tells you what it can check; it does not decide for you and never blocks an action. The report is yours, so keep it honest:

- Split the report into VERIFIED (a real action, its effect observed) and NOT VERIFIED.
- If you replaced the user's action with a JavaScript interceptor (a patched `fetch`, `XMLHttpRequest` or anchor click), say so on the first line of the report. A result seen through an interceptor shows what the page would do, not what a user's click did.
- A download counts only if the file is in the downloads folder. An HTTP answer or the bytes of a file you built yourself do not prove it.
- An HTTP status 0, a 5xx or a network error is a backend failure until shown otherwise: read the backend log before reporting the screen as broken or fine.
- A feature is not finished until it is tested, and the next one does not start before.

What webscope already marks: when `fetch`, `XMLHttpRequest.send` or the anchor `click` are replaced after `page(load)`, the answers of `page` start with `Interceptor: …` (it is a label; nothing is refused). The rest (file seen in the downloads folder, evidence states) is planned in the ROADMAP section «Informes honestos»; until each version exists, those rules are practice, not something the server checks.

## Setup

Set the variables once, as in the README: `WEBSCOPE_DB_URL` (required), `WEBSCOPE_APP_URL` (`page`, exact origin of the tab; several origins separated by commas when the old and the new app answer on different addresses), `WEBSCOPE_BACKEND` and `WEBSCOPE_FRONT` (`code`), `WEBSCOPE_LEGACY` (`code legacy`, the folder of the legacy PHP), `WEBSCOPE_TENANT_QUERY` (a tenant lookup other than Hyn Tenancy's), `WEBSCOPE_DOWNLOADS` (where Chrome saves files, for `page download`), `WEBSCOPE_LOG` (the backend log: `click`, `press` and `payloads` add the error lines written since the click or key began, when a request ends with status 0 or 5xx). A missing variable fails with `<NAME> is not set` and does not guess; `fields` without `WEBSCOPE_BACKEND` still answers, but without readers and writers.
