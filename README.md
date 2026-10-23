# webscope 0.2

Read-only SQL for AI agents over MCP. One tool, `query`, answers as short TSV (header, `\N` for NULL, rows and cells capped) so it costs few tokens.

## Build and register (Claude Code)

    go build -o webscope .        # Go 1.27 or newer
    claude mcp add webscope -e WEBSCOPE_DB_URL='mysql://user:pass@127.0.0.1:3306/dbname' -- /absolute/path/to/webscope

`WEBSCOPE_DB_URL` is `mysql://user:password@host:port/db` or `sqlite:///path/file.db`. Percent-encode special characters in the password.

## The tool

`query(sql, limit?)`: one statement that starts with SELECT, SHOW, DESCRIBE, EXPLAIN or WITH. Two layers keep it read-only: a statement check (one statement, no `INTO`, no `/*! */` comments) and a read-only session (MySQL `READ ONLY` transaction, SQLite `query_only`). Answers stop at 50 rows (at most 500), 200 characters per cell and 8000 bytes, and say when something was left out.

With `tenant` (an fqdn) the query runs in that tenant's database, found in the central database of `WEBSCOPE_DB_URL` with Hyn Tenancy's `hostnames` and `websites` tables. `WEBSCOPE_TENANT_QUERY` replaces that lookup: one `?`, the database name in the first column.

## Not in this version

Several tenants in one call, per-tenant database users, PostgreSQL, browser tests, checks between front, API and back. The older, larger project is `webscope-publico`. Checked against MySQL 9.7.1 and SQLite. Test: `go test ./...`
