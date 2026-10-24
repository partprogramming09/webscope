# webscope 0.3

Two MCP tools for AI agents that spend few tokens. `query`: read-only SQL, answered as short TSV (header, `\N` for NULL, rows and cells capped). `page`: test a screen in your Chrome through Claude in Chrome.

## Build and register (Claude Code)

    go build -o webscope .        # Go 1.27 or newer
    claude mcp add webscope -e WEBSCOPE_DB_URL='mysql://user:pass@127.0.0.1:3306/dbname' -- /absolute/path/to/webscope

`WEBSCOPE_DB_URL` is `mysql://user:password@host:port/db` or `sqlite:///path/file.db`. Percent-encode special characters in the password.

## The tool

`query(sql, limit?)`: one statement that starts with SELECT, SHOW, DESCRIBE, EXPLAIN or WITH. Two layers keep it read-only: a statement check (one statement, no `INTO`, no `/*! */` comments) and a read-only session (MySQL `READ ONLY` transaction, SQLite `query_only`). Answers stop at 50 rows (at most 500), 200 characters per cell and 8000 bytes, and say when something was left out.

With `tenant` (an fqdn) the query runs in that tenant's database, found in the central database of `WEBSCOPE_DB_URL` with Hyn Tenancy's `hostnames` and `websites` tables. `WEBSCOPE_TENANT_QUERY` replaces that lookup: one `?`, the database name in the first column.

## The page tool

`page(action, args?)` does not drive the browser: it returns a short JavaScript snippet and you run it with Claude in Chrome's `javascript_tool`; the snippet returns the screen as compact text. Set `WEBSCOPE_APP_URL` to the app's address (`http://localhost:5173`).

1. `page(load)`: one line that loads the helper. A local server on `127.0.0.1` (random port, random token per start, answers only the app's origin, caps what it receives) serves the helper and keeps the requests the page sends. Run it once per page load.
2. `state` (URL, headings, fields, buttons, alerts, errors, table rows), `read`, `fill` (a whole form in one call; it also chooses in Vuetify `v-select` and `v-autocomplete` menus, scrolling long lists, and fails with the rows it saw when the option is missing), `click`, `press`, `simulate` (the next requests to a URL answer an HTTP error, for `fetch` and XMLHttpRequest), each returns a snippet.
3. `payloads`: the requests sent and their answers, answered by the tool itself. `help` lists the arguments.

Arguments enter the snippet as escaped JSON, never as text. Writes happen only through the screen, with your approval, on rows prefixed `ZZ_`. `fill` on a multiple menu adds the option, it does not replace the ones already chosen, and a field that depends on another (Acme's Servicio on Subtipo) needs its own call. Not covered: `v-combobox`, date pickers, menus of other libraries, file uploads, pages whose Content-Security-Policy blocks the helper (all unchecked against the real app). Tests need a Chrome (`WEBSCOPE_CHROME` to point at one); a missing Chrome fails them.

## Not in this version

Several tenants in one call, per-tenant database users, PostgreSQL, checks between front, API and back. The older, larger project is `webscope-publico`. Checked against MySQL 9.7.1 and SQLite. Test: `go test ./...`
