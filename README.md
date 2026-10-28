# webscope 0.5

Four MCP tools for AI agents that spend few tokens. `query`: read-only SQL, answered as short TSV (header, `\N` for NULL, rows and cells capped). `page`: test a screen in your Chrome through Claude in Chrome. `check`: compare one saved row with what the screen sent and read back. `code`: read the project's code (`fields` of a table, `consumers` of a column, `map` of a front module, `map_back` from a screen to the table) and see what an action on the screen wrote (`snap`, `snap_diff`).

## Build and register (Claude Code)

    go build -o webscope .        # Go 1.27.3 or newer (go.mod)
    claude mcp add webscope -e WEBSCOPE_DB_URL='mysql://user:pass@127.0.0.1:3306/dbname' -- /absolute/path/to/webscope

`WEBSCOPE_DB_URL` is `mysql://user:password@host:port/db` or `sqlite:///path/file.db`. Percent-encode special characters in the password. `WEBSCOPE_BACKEND` is the folder with the project's PHP (the `code` tool needs it; `WEBSCOPE_FRONT` is the folder of the Vue front, used by `map` and `map_back`: its `src/modules` is read if it exists).

## The tool

`query(sql, limit?)`: one statement that starts with SELECT, SHOW, DESCRIBE, EXPLAIN or WITH. Two layers keep it read-only: a statement check (one statement, no `INTO`, no `/*! */` comments) and a read-only session (MySQL `READ ONLY` transaction, SQLite `query_only`). Answers stop at 50 rows (at most 500), 200 characters per cell and 8000 bytes of rows (plus a one-line footer), and say when something was left out. The statement is read under the rules of MySQL and of SQLite at once and must be one safe read under both, so a quote escaped with a backslash (`'it\'s'`) is refused: double it (`'it''s'`). `LOAD_FILE`, `GET_LOCK`, `PRAGMA`, `ATTACH` and `DETACH` are refused too, and so are `FOR UPDATE` and `FOR SHARE` (they take locks) and a `WITH` that ends in `DELETE`, `UPDATE`, `INSERT` or `REPLACE` as a statement (`REPLACE(...)` and `INSERT(...)` as functions are reads). A SQLite file is opened read-only (`mode=ro`): a path that does not exist is an error and is not created.

With `tenant` (an fqdn) the query runs in that tenant's database, found in the central database of `WEBSCOPE_DB_URL` with Hyn Tenancy's `hostnames` and `websites` tables. `WEBSCOPE_TENANT_QUERY` replaces that lookup: one `?`, the database name in the first column.

## The page tool

`page(action, args?)` does not drive the browser: it returns a short JavaScript snippet and you run it with Claude in Chrome's `javascript_tool`; the snippet returns the screen as compact text. Set `WEBSCOPE_APP_URL` to the address of the tab, exactly as the browser writes it: scheme, host and port (`http://localhost:5173`; a tenant on a subdomain is another origin, `http://demo.localhost:5173`). The listener refuses every other origin, so with a different address the helper still loads and `state` works, but `payloads` shows no requests; it then says that a request was refused. Restart the server after changing the variable.

1. `page(load)`: one line that loads the helper. A local server on `127.0.0.1` (random port, random token per start, answers only the app's origin, caps what it receives) serves the helper and keeps the requests the page sends. Run it once per page load.
2. `state` (URL, headings, fields, buttons, alerts, errors, table rows), `read`, `fill` (a whole form in one call; it also chooses in Vuetify `v-select` and `v-autocomplete` menus, scrolling long lists, and fails with the rows it saw when the option is missing), `click` and `press` (both wait for the answer and report the requests with their status, the new alerts and field errors, a URL change, or «No effect seen»), `simulate` (the next requests to a URL answer an HTTP error, for `fetch` and XMLHttpRequest), each returns a snippet.
3. `payloads`: the requests sent and their answers, answered by the tool itself. `help` lists the arguments.

Arguments enter the snippet as escaped JSON, never as text. Writes happen only through the screen, with your approval, on rows prefixed `ZZ_`. `fill` on a multiple menu adds the option, it does not replace the ones already chosen, and a field that depends on another (Acme's Servicio on Subtipo) needs its own call. Not covered: `v-combobox`, date pickers, menus of other libraries, file uploads, pages whose Content-Security-Policy blocks the helper (all unchecked against the real app). Tests need a Chrome (`WEBSCOPE_CHROME` to point at one); a missing Chrome fails them.

## The check tool

`check(sent, table, where, map, after?, at?, tenant?)` compares three things for one saved row: what the screen **sent** (a request `page` captured, e.g. `POST /api/quotes`, the last one that matches), what the database **stored** (the row `where` picks, `{column: value}`, in a read-only transaction) and what the screen **read** afterwards (`after`, another captured request such as `GET /api/quotes`, and `at`, the path of the object inside its answer: `quote`, `data.0`). `map` says which column holds each sent field: `{"sub_type": "SUBTIPO_OPER"}`. Nothing is guessed, because the names differ on each side.

The answer is the counts (equal, `empty~`, different) and one line only for each field that differs, each value cut at 40 characters; a row is never shown whole. `007` is not `7`; a `null`, an empty text and a database NULL count as `empty~` (equivalent, but said so); a field missing from the sent body is `(absent)` and counts as different. A field that the read does not name, while sent and stored agree, is `unread~`: not different (the read says nothing about it) and not equal either, so it is listed apart; if sent and stored differ, it is different and the read shows `(absent)`. A body that was cut when it was captured (`page` keeps 8000 characters of each, the server 16 KiB) is refused, not compared. `check` never writes: it reads captures and one row.

In Acme (development), after saving a quote and deleting it from the screen, `check` shows a single difference: `status  ENPROCESO  BORRADO`. The delete is logical: the row stays, marked `BORRADO`, with its child rows.

## The code tool

`code(action, args?)` reads, never writes. The arguments go in `args` or at the top level of the call (`{"action":"fields","table":"x"}`); a key in `args` wins. `fields {table, tenant?}` is the sheet of a table: per column its type, null, default, how many rows have a value (`3/54`) and how many PHP files read and write it (`14 / 7`); a column with no value in any row, or that no file names, is marked. It returns counts, never a stored value, and stops at 8000 bytes. `consumers {table, column}` (or `{column_only}` to search the whole backend) lists who reads and who writes a column, `file:line`, from the files that name the table or its model (the one that declares `$table = '...'`) and from the `Resources/` folder of the model's module, whose classes read the columns as `$this->COLUMN` without naming either.

Both are estimates by text, and the answer says so: a use is `WRITES` when its line assigns the column (`$x->COL = v`, `.=`, `??=`) or holds it as the key of an array (`'COL' => v`) within 3 lines of an `update`, `create`, `save`, `insert`, `fill` or `upsert` call, `READS` otherwise (a comparison, a filter or a value read into an array next to a write call is a read). The column is searched as text, not as a regular expression. Only the `.php` files under `WEBSCOPE_BACKEND` are read: symbolic links are never followed, and `.env*`, `vendor`, `node_modules`, `storage` and `.git` are left out.

In Acme (development), `fields app_cotizaciones` gives the 42 columns of the table, with the same counts as `query` (54 rows, `ESTADO` 54, `DESCRIPCION` 53, `MOTIVO_RECHAZO` 5); `consumers app_cotizaciones MOTIVO_RECHAZO` finds the 7 uses in the 5 files that a `grep` of the whole backend finds (the two Resources among them), and `consumers app_cotizaciones ESTADO` finds 51 uses in 23 of the 77 files it searches. Walking 2212 PHP files takes about 0.3 s of CPU.

`map {module, walkthrough?}` lists, for the front modules whose path contains `module` (e.g. `crm/quotes`): the routes with their view and `file:line`, the HTTP calls (`axios`, `http` or `api` with a URL in quotes, on the same line or the next), the composables and the stores. With `walkthrough` it lists instead what can be clicked: actions with their `v-if`, permissions (`can(...)`), links, form fields (`name*` is required), tabs and statuses. `map_back {query}` takes a front module or a part of an API address and gives each endpoint with its controller and the line of the method, the front call that uses it, the services and the models with their table. The routes are deduced from `app/Modules/**/Routes/*.php`; no `artisan` is run, and `--required`, `--seen`, `--coverage` and the saved walkthroughs of the old project are not here. Answers stop at 8000 bytes; when the endpoints of `map_back` do not fit, the last endpoints go (with a line that says how many) and the services and tables stay.

In Acme (development), `map crm/quotes` gives 2 routes and 53 calls, the same as a `grep` of the module (53 lines with a call), including the calls whose URL is on the next line. `map_back crm/quotes` finds 47 endpoints and their controllers with the line of each method.

`snap {name, sql, key?, tenant?}` keeps in memory the result of a read-only query, by a key column (the first by default; up to 2000 rows and 20 snapshots), and `snap_diff {name}` repeats the query and shows only what changed: `new KEY: col=value`, `deleted KEY` and, for a changed row, each column as `before → after` (NULL is `\N`, an empty text `''`, a value is cut at 40 characters). It answers "what did this action really write", for example the child rows that saving a quote leaves. The snapshots are lost when the server restarts, and a text value that is exactly `\N` reads as NULL. Against Acme (development), a snapshot of the 54 quotes followed by a diff with nothing done says `54 rows before, 54 now: no changes`.

## Docs

`docs/module-testing.md` and `docs/patterns.md` are the procedure for testing a module and the list of patterns found, written for the old server and copied unchanged; `docs/old-to-new-tools.md` says which tool of this server replaces each tool they name, and which ones no longer exist.

## Known limits

`consumers`, `fields` and `map_back` read at most 20000 `.php` files of 2 MB each, and they stop there without saying so. `map` reads a generic up to its first `>`, so a type with `=>` inside is not recognized. `fields` counts the values of every column with one scan of the table: fine for the tables of an application, slow on a table of millions of rows. `snap` keeps a value of up to 4096 bytes whole and a longer one cut plus its hash, so a change anywhere in it still shows. `check` shows a text that differs only in spaces as `space~`, not as equal. A tool that fails with a bug answers `Failed: internal error` and the server keeps running. The code is searched by text, not parsed: every answer of `code` is an estimate. The address of the app (`WEBSCOPE_APP_URL`) must be ASCII: a host with accents does not match the punycode that the browser sends, and `page` refuses it (it fails closed).

## Not in this version

Several tenants in one call, per-tenant database users, PostgreSQL, a mapping of fields that is found by itself (`check` takes it from you), the old `--required`, `--seen` and `--coverage` options of `map` and `map_back`. The older, larger project is `webscope-publico`. Checked against MySQL 9.7.1 and SQLite. Test: `go test ./...`
