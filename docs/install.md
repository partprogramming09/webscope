# Installing webscope in an agent

**Reader:** people who want to connect webscope to Claude Code or to another MCP client.

webscope is one stdio MCP server. Any client that can start a command and talk to it over stdio can use it; what changes is where each client keeps its configuration. The environment variables are the same everywhere (README, «Quick start»); only `WEBSCOPE_DB_URL` is needed to start.

Sources and confidence of every format below: `docs/research/2026-11-12-mcp-marketplace-and-sdk.md`.

## Claude Code, as a plugin (recommended)

    claude plugin marketplace add partprogramming09/webscope
    claude plugin install webscope@webscope

`/plugin configure webscope@webscope` asks for `db_url`, `app_url`, `backend`, `front` and `legacy`; an empty one just leaves its tool off. The database URL is stored as a sensitive value. Do not pass it with `--config` on the command line: it stays in the shell history and shows in `ps`.

- The plugin brings the server and four skills (`module-flows`, `migration-parity`, `parity-tests`, `screen-audit`).
- The first start builds the binary with `go build` (Go as in `go.mod` must be installed); it took 7.6 to 9.1 s with Go's module cache warm and 12.8 to 14.4 s with it empty (3 runs each, one machine), and `bin/webscope-launch build` does it ahead of time if the client's startup timeout is short (its default was not verified), and it is kept per version; it is rebuilt when a `.go` file is newer than it, so a `git pull` is picked up. To use a binary you built, set `WEBSCOPE_BIN` to its path.
- The repository is private: the machine needs git access to it (SSH key, or `gh auth login` and `gh auth setup-git`).
- Tool names inside Claude Code are `mcp__plugin_webscope_webscope__<tool>`.

## Claude Code, without the plugin

    go build -o webscope .
    claude mcp add -s user webscope -e WEBSCOPE_DB_URL='…' -- /absolute/path/to/webscope

## Other clients

All of them start `/absolute/path/to/webscope` with the variables as environment. **The database URL holds a password: never write it literally in a file that is committed** (`.cursor/mcp.json`, `.mcp.json`, `.vscode/mcp.json`). Put it in the client's user-level configuration, or reference an environment variable where the client allows it (Cursor: `${env:WEBSCOPE_DB_URL}`; the other clients' syntax was not verified here).

| Client | Where | Shape | Status |
|---|---|---|---|
| Cursor | `.cursor/mcp.json` or `~/.cursor/mcp.json` | `{"mcpServers":{"webscope":{"command":"/path/webscope","env":{"WEBSCOPE_DB_URL":"…"}}}}`; values can use `${env:NAME}` | read in the docs, not tried here |
| VS Code | workspace `.mcp.json` or `~/.copilot/mcp-config.json` | `{"mcpServers":{"webscope":{"type":"stdio","command":"/path/webscope","env":{…}}}}` | read in the docs, not tried here |
| Gemini CLI | `~/.gemini/settings.json` or `.gemini/settings.json` | `{"mcpServers":{"webscope":{"command":"/path/webscope","env":{…}}}}`; no underscores in the name | read in the docs, not tried here |
| Codex CLI | `~/.codex/config.toml` or `.codex/config.toml` | `[mcp_servers.webscope]` with `command = "/path/webscope"` and a `[mcp_servers.webscope.env]` table | summary of the docs, not verbatim; not tried here |

Not done: a Codex plugin or marketplace (its file format has not been read), Windsurf (the configuration path is not confirmed), and publishing in the MCP Registry (`server.json` would need a release with a `.mcpb` file and its SHA-256; see `docs/adr/0002-plugin-builds-the-binary.md`).

`AGENTS.md` at the root of the repository tells any agent that opens it what the four tools are for and the rules that hold.

## Checking it works

Ask the agent for `query` with `SELECT 1` and for `page(help)`. In Claude Code, `claude mcp list` shows the server as connected.
