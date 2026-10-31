# webscope

An MCP server that lets an AI agent test a web application and understand its data and code while spending very few tokens. It reads your database, looks at and drives a screen in your Chrome, checks that what a screen sent is what ended up saved, and reads the project's code. One Go binary, four tools, and it never writes to your database.

## What it is for

When you ask an agent to «test the quotes screen», the expensive part is not thinking: it is reading. Whole pages of HTML, hundreds of table rows, the output of a `grep` over a backend. webscope answers with short, capped, structured text instead, so the agent can do the whole round (look at the screen, fill it, save, see what was stored, see what else changed) in a handful of calls.

It was built against a real Laravel + Vue application and its older PHP version, and works on any web app and any MySQL or SQLite database. Some pieces are tied to a stack: `map`, `map_back`, `flows` and `cases` to Laravel routes and Vue modules, and `legacy` to legacy PHP with the SQL inside the file and jQuery calls; the rest is generic.

## What you can ask your agent

With the server registered, ask in plain words:

- «How many rows does `quotes` have, and which columns are always empty?» (`query`, `code fields`)
- «Who reads or writes the column `status` of `quotes` in the backend?» (`code consumers`)
- «Open the quotes screen, fill the form with test data, save it, and tell me if what the screen sent is what was stored.» (`page`, `check`)
- «What did saving that quote really write? List every row it touched, child tables included.» (`code snap`, `snap_diff`)
- «Make the quotes list answer a 500 and tell me what the screen shows.» (`page simulate`)
- «Audit this screen: text that was not rendered, controls covered by something, fields nobody can name.» (`page audit`)
- «What does the old screen send when I press Search?» (`page payloads`, also on legacy pages that use jQuery)
- «Does the new screen send and receive the same fields as the old one?» (`page parity {req_a, req_b}`)
- «Compare the old and the new version of this screen.» (`page shape` on each, then `page parity`; the `screen-audit` skill, see below)
- «Which endpoint, controller and table does this screen use?» (`code map_back`)
- «What are the flows and user stories of this module?» (`code flows`, then the `module-flows` skill)
- «What should I test in this module, and does the migration keep what the legacy did?» (`code cases`, then the `migration-parity` skill)
- «Which unit tests pin what the legacy did?» (the `parity-tests` skill, which hands a plan to the `tester` agent)

## The four tools

| Tool | In one line | It needs |
|---|---|---|
| `query` | One read-only SQL statement; the answer is a short table. | `WEBSCOPE_DB_URL` |
| `page` | Read, fill and click a screen in your Chrome, see the requests it sent, force an HTTP error, audit it, and compare an old screen and a new one. | Claude in Chrome and `WEBSCOPE_APP_URL` |
| `check` | For one saved row: what the screen sent, what the database stored, what the screen read back. Only the differences. | `page` and the database |
| `code` | The sheet of a table, who uses a column, the routes and calls of a screen, what an action changed, the flows and test cases of a module, and what a legacy PHP module calls and writes. | `WEBSCOPE_BACKEND`, `WEBSCOPE_FRONT`, `WEBSCOPE_LEGACY` |

A typical session, in the order that works best (`docs/usage.md` has the contract of each tool):

1. `code fields` and `query`: learn the table and its current counts.
2. `code map` or `code map_back`: learn which endpoints and tables the screen touches.
3. `code snap`: keep the table as it is before the action.
4. `page`: `load`, `state`, `fill`, `click`.
5. `check`: compare the saved row with what was sent and read back.
6. `code snap_diff`: see every row the action wrote.
7. Clean up through the screen, never through the database.

## Quick start (Claude Code)

    go build -o webscope .        # Go 1.27.3 or newer (go.mod)
    claude mcp add webscope \
      -e WEBSCOPE_DB_URL='mysql://user:pass@127.0.0.1:3306/dbname' \
      -e WEBSCOPE_APP_URL='http://localhost:5173' \
      -e WEBSCOPE_BACKEND='/path/to/backend' \
      -e WEBSCOPE_FRONT='/path/to/front' \
      -- /absolute/path/to/webscope

Only `WEBSCOPE_DB_URL` is needed to start (`query`). The others switch on the rest:

- `WEBSCOPE_APP_URL`: the address of the tab to test, exactly as the browser writes it (scheme, host and port). `page` needs it, and it needs the Claude in Chrome extension. When the old and the new app answer on different addresses, list them separated by commas (`http://localhost:8088,http://demo.localhost:5173`): the helper then works in a tab of any of them and the requests of all of them are kept together.
- `WEBSCOPE_BACKEND`: the folder with the project's PHP. `code` uses it.
- `WEBSCOPE_FRONT`: the folder of the Vue front (its `src/modules` is read if it exists). `code map` and `map_back` use it.
- `WEBSCOPE_LEGACY`: the folder of the legacy PHP (the one that holds the module folders, for example `public_html`). `code legacy` reads it; it never leaves it, follows no link and opens no `.env*`.
- `WEBSCOPE_TENANT_QUERY`: a tenant lookup other than Hyn Tenancy's (see below).

After changing a variable, or after rebuilding, reconnect with `/mcp`: the connected client keeps talking to the old process. To check it works, ask the agent for `query` with `SELECT 1` and for `page(help)`.

## What it guarantees

- **It never writes to the database.** `query` is read-only twice (a statement check, and a read-only session); `check`, `code` and `snap` only read.
- **Writes happen only through the screen**, with your approval, on rows prefixed `ZZ_`. Cleaning up is done through the screen too.
- **Answers are small and say what they left out**: 50 rows (at most 500), 200 characters per cell, 8000 bytes per answer. The list of tools the agent has to load is 1443 bytes.
- **`page` is local**: a server on `127.0.0.1` with a random port and token, which answers only the origin of your app.
- **No guessing**: `check` takes the field-to-column map from you, and every answer of `code` that comes from searching text says it is an estimate.
- With customer data, ask for the schema and for counts, not for rows.
- **The tool reports, the model decides**: webscope only labels facts it can check and never blocks an action. How to report a test honestly (verified apart from not verified, an interceptor said on the first line, a download counts only if the file is on disk) is in [docs/usage.md](docs/usage.md#reporting-a-test).

## Things worth taking advantage of

- **Tenants.** `tenant` on `query`, `check` and `code fields` runs in another database, found in the central one (Hyn Tenancy by default).
- **Errors on demand.** `page simulate {match, status}` makes the next requests to a URL answer an HTTP error, so you see what the screen does with a 500 without breaking anything.
- **What did the action really write?** `snap` before and `snap_diff` after show only what changed, child rows included.
- **Old pages too.** The helper sees `fetch` and `XMLHttpRequest`, including pages that replace the constructor (the legacy PHP pages of the app that motivated this); a legacy link reloads the page, so run `page(load)` again after each one.
- **A review of a screen without a reference.** `page audit` lists unrendered text, covered controls, unlabeled fields, repeated labels and unnamed buttons; it is a hint, not a verdict.
- **The `screen-audit` skill** (`.claude/skills/screen-audit/SKILL.md`): how to review a screen, how to look for fields that are missing or should not be there, and how to compare an old and a new screen with these tools.
- **A list of what has been found.** `docs/patterns.md` records the findings met while testing screens, each with how to recognize it, what to do and how far it was verified. `docs/module-testing.md` is the procedure for testing a whole module.

## Migrating a module from a legacy system, start to end

The case this was extended for: a module that exists in an old system and was rebuilt, and you want to know what the new one lacks, adds or changes. It takes two origins (`WEBSCOPE_APP_URL=http://old:8088,http://new:5173`), the legacy folder (`WEBSCOPE_LEGACY`) and two tabs of Chrome with the session started. Everything below reads; the only writes are one test row per system through the screen, prefixed `ZZ_`, and only with your approval.

| Step | Ask the agent | What it runs | What comes back |
|---|---|---|---|
| 1. What the module does | «Give me the flows and user stories of `crm/quotes`» | `code flows`, skill `module-flows` | Screens and actions with their conditions and permissions, then endpoints, controllers and tables, as evidence; the stories are written from it and you confirm them |
| 2. What the old code touches | «What does the legacy quotes module call and write, and which tables does the new one not have?» | `code legacy {path, module}` | Each legacy endpoint with its caller and tables; the tables only one side has, split into «named elsewhere in the backend» and «not named» |
| 3. Screens | «Compare the quotes list of the old and the new screen» | `page shape {name, selector}` on each, `page parity {a, b}` | Only the fields, buttons and columns one side has |
| 4. Requests | «Does the new search send and receive what the old one did?» | the same action on both, `page parity {req_a, req_b, map}` | Status and the field names only one side sends or gets, never values |
| 5. What to test | «List the test cases for this module» | `code cases` | A table: happy path, permission, required field, status, HTTP error and empty list, with what runs each |
| 6. A write on both | «Create a `ZZ_` quote on each system and compare what was saved» | `code snap`, `page fill` and `click`, `code snap_diff`, `code rows`, `query` | The rows each system wrote, their differences, and the child rows by count |
| 7. Unit tests | «Plan the unit tests that pin the old behaviour» | skill `parity-tests`, agent `tester` | A plan by layer, with which tests must fail today |

The whole chain is the `migration-parity` skill. A real run on a quotes module found, among other things: 21 of 22 tables «only in the legacy» were in fact used by the new backend, one database view the old code reads does not exist, the old screen was saving a number the new one had already taken (`Duplicate entry … 1062`), and the two systems wrote the same child rows for a new quote. `CHANGELOG.md` has the detail of each run and what was not checked.

## Docs

`docs/usage.md` is the contract of each tool and the order to use them in. `docs/module-testing.md` is the procedure for testing a module (written for the old server and copied unchanged) and `docs/patterns.md` the list of patterns found (started with the old server's and extended with what each session found since); `docs/old-to-new-tools.md` says which tool of this server replaces each tool they name, and which ones no longer exist. `CHANGELOG.md` has one entry per version.

---

# Reference

## The query tool

`query(sql, limit?)`: one statement that starts with SELECT, SHOW, DESCRIBE, EXPLAIN or WITH. `WEBSCOPE_DB_URL` is `mysql://user:password@host:port/db` or `sqlite:///path/file.db`; percent-encode special characters in the password. The answer is TSV (header, `\N` for NULL, rows and cells capped).

Two layers keep it read-only: a statement check (one statement, no `INTO`, no `/*! */` comments) and a read-only session (MySQL `READ ONLY` transaction, SQLite `query_only`). Answers stop at 50 rows (at most 500), 200 characters per cell and 8000 bytes of rows (plus a one-line footer), and say when something was left out. The statement is read under the rules of MySQL and of SQLite at once and must be one safe read under both, so a quote escaped with a backslash (`'it\'s'`) is refused: double it (`'it''s'`). `LOAD_FILE`, `GET_LOCK`, `PRAGMA`, `ATTACH` and `DETACH` are refused too, and so are `FOR UPDATE` and `FOR SHARE` (they take locks) and a `WITH` that ends in `DELETE`, `UPDATE`, `INSERT` or `REPLACE` as a statement (`REPLACE(...)` and `INSERT(...)` as functions are reads). A SQLite file is opened read-only (`mode=ro`): a path that does not exist is an error and is not created.

With `tenant` (an fqdn) the query runs in that tenant's database, found in the central database of `WEBSCOPE_DB_URL` with Hyn Tenancy's `hostnames` and `websites` tables. `WEBSCOPE_TENANT_QUERY` replaces that lookup: one `?`, the database name in the first column.

## The page tool

`page(action, args?)` does not drive the browser: it returns a short JavaScript snippet and you run it with Claude in Chrome's `javascript_tool`; the snippet returns the screen as compact text. Set `WEBSCOPE_APP_URL` to the address of the tab, exactly as the browser writes it: scheme, host and port (`http://localhost:5173`; a tenant on a subdomain is another origin, `http://demo.localhost:5173`). The listener refuses every other origin, so with a different address the helper still loads and `state` and `audit` work, but `payloads` shows no requests; it then says that a request was refused. Restart the server after changing the variable. After a rebuild and `/mcp`, a page that still holds the helper of the old build answers `Failed: an older helper is in this page`: reload the page and run `page(load)` again.

1. `page(load)`: one line that loads the helper. A local server on `127.0.0.1` (random port, random token per start, answers only the app's origin, caps what it receives) serves the helper and keeps the requests the page sends. Run it once per page load, and again after every navigation, reload or `/mcp`.
2. `state` (URL, title, the console lines logged since the helper loaded (`console.error`, `console.warn`, uncaught errors and rejected promises, without query strings; what was logged before `load` is not seen), headings, fields, buttons (the dialog's first, the side menu's last, up to 30, disabled ones marked `(disabled)`), alerts, errors, table rows), `read`, `fill` (a whole form in one call; when a name fits several fields, such as a list filter and a form field with the same label, it fills the first one (a dialog's before the page's), warns and lists the others, and an id picks one; it also chooses in Vuetify `v-select` and `v-autocomplete` menus, scrolling long lists, and fails with the options it saw when the one asked for is missing, in a menu or in a native `<select>`), `click` and `press` (both wait for the answer and report the requests (method, path without the query string, status, or `0 (no answer)` for one nobody answered), the new alerts, field errors and console errors, a URL change, or «No effect seen»; they and `state` also warn when the tab's origin is not the one in `WEBSCOPE_APP_URL`), `simulate` (the next requests to a URL answer an HTTP error, for `fetch` and XMLHttpRequest), `audit` (what the page shows wrong about itself, see below), each returns a snippet.
3. `payloads`: the requests sent and their answers, answered by the tool itself. `help` lists the arguments.

`fill` also writes into a rich-text editor (`contenteditable`), found by its `aria-label` or id or, when it has neither, as `editor`. A field whose `<label>` is empty is named by its `aria-label`, `placeholder`, `name` or `id`. `click {text}` falls back to the plain element with exactly that text when no button or link has it, and when nothing matches the error lists the dialog's buttons first.

`shape {name, selector?}` saves what the screen offers under a name (with `selector`, only what is inside the first element it matches, such as the list or a dialog; a selector that matches nothing fails and saves nothing), in the local listener and in memory (field labels and kinds, buttons, table columns, tabs and headings; no values; the side menu and the top bar are left out). `parity {a, b}` (answered by the server, no snippet) compares two saved shapes and prints only `only in a`, `only in b` and the fields that kept their label but changed kind; names are compared ignoring case and spaces. It compares names, so it cannot say two screens behave the same. Shapes are lost when the server restarts; up to 20 are kept.

`parity {req_a, req_b, map?}` compares instead two captured requests, for example the same action on the old and the new screen: each spec is a method and a part of the address (`POST save.php`), and the latest capture that matches is used. It prints the two requests, the status if it differs, and the field names only one side sent (`sent only in a`) or received (`got only in b`); never a value. It reads JSON (nested names with a dot, a list by its first element as `lines[].qty`), forms (`a=1&b=2`, what the old jQuery pages send) and the query string of the address (a `GET` sends its fields there; a list index is written `[]`, as in `filters[statuses][]`). The request line shows the address without its query string and how many parameters it had (`?5 params`), never their values; a body that was cut or that is neither says so and is not compared. `map {name in a: name in b}` renames the names of a first, so `DESCRIPCION` and `notes` count as the same field.

`audit {selector?}` lists, without any reference screen: unrendered text (`{{ x }}`, `undefined`, `null`, `NaN`, `[object Object]`), controls covered by another element, fields with no label (with their `name`, `id` and a short CSS path), repeated labels, and buttons or links with no name. It says so when it finds nothing, and `selector` limits it to a part of the screen. It is a hint to look at, not a verdict.

`download` proves a download with the real click: `download {arm:true}` notes the files in the downloads folder (`WEBSCOPE_DOWNLOADS`, else `~/Downloads`), you make the real click in the page, and `download {wait:true, seconds?}` (10 s by default, at most 60) returns the new file's name, size, type by its first bytes (`PK` with `xl/workbook.xml` is xlsx, `%PDF` is pdf) and time, and for an xlsx the header names and the row count, never a value. Partial files (`.crdownload`) do not count. If no file appears, it answers `Failed: no file appeared …`: an export is downloaded only if this finds the file, not because a replaced `fetch` or a click on a patched anchor returned the right bytes. When the page's `fetch`, `XMLHttpRequest.send` or anchor `click` were replaced after `load`, the answers of `page` start with an `Interceptor:` line (a label; nothing is refused).

`verify` builds the final report from evidence: `verify {claim, evidence}` adds a claim and answers its state, which the server decides, not the caller. `evidence` is a reference: `capture POST /api/quotes` (a request that `page` captured) or `download` (a file that `download wait` found). `OBSERVADO` means the reference exists; `INFERIDO` means it exists but an interceptor was seen on the page since `load`; `NO VERIFICADO` means there is no evidence or none the server can find (free text is not evidence). `verify {report:true}` prints the table and `{clear:true}` empties it. It is optional and blocks nothing.

Arguments enter the snippet as escaped JSON, never as text. Writes happen only through the screen, with your approval, on rows prefixed `ZZ_`. `fill` on a multiple menu adds the option, it does not replace the ones already chosen, and a field that depends on another needs its own call. Not covered: `v-combobox`, calendar-style date pickers, menus of other libraries, file uploads, an editor inside an iframe (TinyMCE), pages whose Content-Security-Policy blocks the helper (all unchecked against the real app). Tests need a Chrome (`WEBSCOPE_CHROME` to point at one); a missing Chrome fails them.

## The check tool

`check(sent, table, where, map, after?, at?, tenant?)` compares three things for one saved row: what the screen **sent** (a request `page` captured, e.g. `POST /api/quotes`, the last one that matches), what the database **stored** (the row `where` picks, `{column: value}`, in a read-only transaction) and what the screen **read** afterwards (`after`, another captured request such as `GET /api/quotes`, and `at`, the path of the object inside its answer: `quote`, `data.0`). `map` says which column holds each sent field: `{"sub_type": "SUBTIPO_OPER"}`. Nothing is guessed, because the names differ on each side.

The answer is the counts (equal, `empty~`, different) and one line only for each field that differs, each value cut at 40 characters; a row is never shown whole. `007` is not `7`; a `null`, an empty text and a database NULL count as `empty~` (equivalent, but said so); a field missing from the sent body is `(absent)` and counts as different. A field that the read does not name, while sent and stored agree, is `unread~`: not different (the read says nothing about it) and not equal either, so it is listed apart; if sent and stored differ, it is different and the read shows `(absent)`. A body that was cut when it was captured (`page` keeps 8000 characters of each, the server 16 KiB) is refused, not compared. `check` never writes: it reads captures and one row.

## The code tool

`code(action, args?)` reads, never writes. The arguments go in `args` or at the top level of the call (`{"action":"fields","table":"x"}`); a key in `args` wins. `fields {table, tenant?}` is the sheet of a table: per column its type, null, default, how many rows have a value (`3/54`) and how many PHP files read and write it (`14 / 7`); a column with no value in any row, or that no file names, is marked. It returns counts, never a stored value, and stops at 8000 bytes. `consumers {table, column}` (or `{column_only}` to search the whole backend) lists who reads and who writes a column, `file:line`, from the files that name the table or its model (the one that declares `$table = '...'`) and from the `Resources/` folder of the model's module, whose classes read the columns as `$this->COLUMN` without naming either.

Both are estimates by text, and the answer says so: a use is `WRITES` when its line assigns the column (`$x->COL = v`, `.=`, `??=`) or holds it as the key of an array (`'COL' => v`) within 3 lines of an `update`, `create`, `save`, `insert`, `fill` or `upsert` call, `READS` otherwise (a comparison, a filter or a value read into an array next to a write call is a read). The column is searched as text, not as a regular expression. Only the `.php` files under `WEBSCOPE_BACKEND` are read: symbolic links are never followed, and `.env*`, `vendor`, `node_modules`, `storage` and `.git` are left out.

`map {module, walkthrough?}` lists, for the front modules whose path contains `module` (e.g. `crm/quotes`): the routes with their view and `file:line`, the HTTP calls (`axios`, `http` or `api` with a URL in quotes, on the same line or the next), the composables and the stores. With `walkthrough` it lists instead what can be clicked: actions with their `v-if`, permissions (`can(...)`), links, form fields (`name*` is required), tabs and statuses. `flows {module}` joins both in one answer (screens with their actions, then endpoints, controllers and tables) as evidence for user stories; without `WEBSCOPE_BACKEND` it gives the screens and says so. `legacy {path, module?}` reads a folder of the legacy PHP (`path` is relative to `WEBSCOPE_LEGACY`, e.g. `comercial/cotizadorX_new`) and lists each PHP file that some page calls, with the first caller (`file:line`, and `(+N)` more), the tables its SQL writes (`W`) and the ones it only reads (`R`), and the files nobody calls. With `module` (a front module such as `crm/quotes`) it adds `only in legacy`, `only in new` and `both`: the tables of the whole legacy folder against the tables the new module's models name. With `WEBSCOPE_BACKEND` set, `only in legacy` is split into the tables that some other PHP file of the backend names (with how many files) and the ones no file names, which are the candidates for a gap; each list stops at 60 names. It is written for legacy PHP with the SQL inside the file and jQuery calls with a quoted `.php` address; it is an estimate by text, reads only the endpoint file (not what it includes: the writes that live in an included file show on that file) and skips comment lines and `ON DUPLICATE KEY UPDATE`. `cases {module}` turns those flows into a table of test cases (`id`, `flow`, `case`, `run with`, `verify with`): the happy path of each action, the permission that guards it, each required field left empty, the statuses, an HTTP error on each write endpoint (`page simulate`) and an empty list for each list endpoint; up to 120 rows. It runs nothing and says that it is a list, not a result; what runs a case is `page`, what verifies it is `check` or `query`. `map_back {query}` takes a front module or a part of an API address and gives each endpoint with its controller and the line of the method, the front call that uses it, the services and the models with their table. The routes are deduced from `app/Modules/**/Routes/*.php`; no `artisan` is run, and `--required`, `--seen`, `--coverage` and the saved walkthroughs of the old project are not here. Answers stop at 8000 bytes; when the endpoints of `map_back` do not fit, the last endpoints go (with a line that says how many) and the services and tables stay.

`snap {name, sql, key?, tenant?}` keeps in memory the result of a read-only query, by a key column, or several separated by commas (the first column by default; up to 2000 rows and 20 snapshots), and `snap_diff {name}` repeats the query and shows only what changed: `new KEY: col=value`, `deleted KEY` and, for a changed row, each column as `before → after` (NULL is `\N`, an empty text `''`, a value is cut at 40 characters). It answers «what did this action really write», for example the child rows that saving a quote leaves. The snapshots are lost when the server restarts, and a text value that is exactly `\N` reads as NULL. A table whose first column repeats needs `key` with a unique column, or a count before and after.

## Measured on a real application

What each tool gave on the development tenant of the application it was built against (a Laravel + Vue app with an older PHP version), with the date of the measurement. Counts change as the data does.

- `query`: 34 tables in the central database and 59 rows in the quotes table (29 Oct).
- `code fields` on the quotes table: 42 columns, the same counts as `query` (`MOTIVO_RECHAZO` has a value in 5 of 59 rows, 29 Oct).
- `code consumers` of that column: 7 uses in the 5 files that a `grep` of the whole backend finds, the two Resources among them (24 and 29 Oct). Of `ESTADO`: 51 uses in 23 of the 77 files it searches (24 Oct). Walking 2212 PHP files takes about 0.3 s of CPU.
- `code map crm/quotes`: 2 routes and 53 calls, the same as a `grep` of the module, including the calls whose URL is on the next line (24 Oct; 2 routes again on 29 Oct). `code map_back crm/quotes`: 47 endpoints with the line of each controller method on 24 Oct; on 29 Oct it showed 11 and said that 50 more were left out by the 8000-byte cap.
- `code snap` then `snap_diff` with nothing done: `59 rows before, 59 now: no changes` (29 Oct). After creating a test quote on the screen: `60 rows before, 61 now` and one new row; the versions table went from 65 to 66 rows (counts).
- `check` after saving a quote and deleting it from the screen: one difference, `status  ENPROCESO  BORRADO`. The delete is logical: the row stays, marked `BORRADO`, with its child rows (28 and 29 Oct).
- `page` on the older PHP version of the screen: «Buscar» on the quotes list reports 4 POSTs to the server's `*.php` files, with what was sent and what came back (29 Oct). `page audit` on the same screen: 3 `select` with no label, `Estado` twice and 14 unnamed buttons and links; on the new version's «Nueva Cotización» dialog it says `No findings` (29 Oct).
- `page` on the new version: a full round (fill 12 fields, save, `check`, delete) works in a real Chrome, with `fill` choosing in the Vuetify menus and the native `date` input (28 Oct).

## Known limits

`consumers`, `fields` and `map_back` read at most 20000 `.php` files of 2 MB each, and they stop there without saying so. `map` reads a generic up to its first `>`, so a type with `=>` inside is not recognized. `fields` counts the values of every column with one scan of the table: fine for the tables of an application, slow on a table of millions of rows. `snap` keeps a value of up to 4096 bytes whole and a longer one cut plus its hash, so a change anywhere in it still shows. `check` shows a text that differs only in spaces as `space~`, not as equal. A tool that fails with a bug answers `Failed: internal error` and the server keeps running. The code is searched by text, not parsed: every answer of `code` is an estimate. The address of the app (`WEBSCOPE_APP_URL`) must be ASCII: a host with accents does not match the punycode that the browser sends, and `page` refuses it (it fails closed). One `WEBSCOPE_APP_URL` per server: to look at a second origin (say, the old and the new version of an app) change the variable and reconnect.

## Not in this version

Several tenants in one call, per-tenant database users, PostgreSQL, a mapping of fields that is found by itself (`check` takes it from you), capturing the SQL the server runs (it would need a database account that can change the server's configuration, against the rule that the server never writes), the old `--required`, `--seen` and `--coverage` options of `map` and `map_back`. The older, larger project is `webscope-publico`. Checked against MySQL 9.7.1 and SQLite. Test: `go test ./...`
