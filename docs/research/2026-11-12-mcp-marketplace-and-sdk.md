# MCP spec, Go SDK and distribution formats (marketplaces and other agents)

**Reader:** whoever packages webscope for Claude Code and other agents, or audits it against the MCP standard.

Consulted on 2026-11-12. Confidence: **A** read verbatim or run here; **B** summarised by the fetch tool, not verbatim; **C** not verified.

## 1. Spec and SDK

| Fact | Source | Conf. |
|---|---|---|
| Current MCP revision is `2026-07-28`; previous `2025-11-25`. It removes `initialize`, makes `server/discover` mandatory, drops sessions (`Mcp-Session-Id`), deprecates roots, sampling and logging. | https://modelcontextprotocol.io/specification/versioning and `/specification/2026-07-28/changelog` | B |
| go-sdk `v1.8.0` is the latest release and the one in `go.mod`. Its `latestProtocolVersion` is `2026-07-28`; it supports `2026-07-28, 2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05`. | `mcp/shared.go` in the module cache (`go-sdk@v1.8.0`), run here | A |
| `ClientOptions.ProtocolVersion` lets a test client pin one revision. | `mcp/client.go` in the module cache | A |
| webscope 0.7.18 answers the old handshake (`initialize` with `2025-06-18`) and the new one (`server/discover` and `tools/list` with `_meta` `io.modelcontextprotocol/protocolVersion` and `clientCapabilities`) over stdio. `server/discover` without `clientCapabilities` gives `-32602`, as the changelog requires. | probes against `./webscope` on 2026-11-12 | A |
| `tools/list` is 1476 bytes in the old era and 1591 in the new one (envelope `resultType`, `_meta`, `ttlMs`, `cacheScope`). Budget in `main_test.go`: 1800. | same probes | A (one run each; the 3-run rule applies when it goes in a report) |
| The server advertises `capabilities.logging`, which `2026-07-28` deprecates. It is the SDK default. | probe output | A |
| Tool errors belong in `Content` with `IsError: true`, not as protocol errors. Log to stderr on stdio. | go-sdk `docs/server.md`, changelog | B |

## 2. Claude Code plugin and marketplace

| Fact | Source | Conf. |
|---|---|---|
| `marketplace.json`: required `name`, `owner.name`, `plugins[]`; each plugin needs `name` and `source`. `source` can be a relative path (`./…`, no `..`), `github`, `url`, `git-subdir`, `npm`, `archive` (with optional `sha256`) or `command`. | https://code.claude.com/docs/en/plugins/marketplace-reference.md | A/B |
| `plugin.json`: only `name` is required. A plugin bundles an MCP server through `.mcp.json` at its root or `mcpServers`. Variables `${CLAUDE_PLUGIN_ROOT}` (changes on update), `${CLAUDE_PLUGIN_DATA}` (persistent) and `${user_config.KEY}` work in `command`, `args` and `env`. `userConfig` entries take `sensitive: true`. Skills go in `skills/` at the plugin root. | https://code.claude.com/docs/en/plugins-reference | A |
| `command` source runs at install, update and once per session, through `sh`, prints one line (the plugin directory), at most 500 printable ASCII characters, and the user must approve the whole command. | marketplace-reference | A |
| A private GitHub repo works as a marketplace with the machine's own git credentials (SSH, or a credential helper such as `gh auth setup-git`); `GITHUB_TOKEN` alone does not authenticate background updates. | https://code.claude.com/docs/en/plugins/host-marketplace.md | A |
| Users get a new copy only when `version` changes (plugin.json wins over the marketplace entry). | host-marketplace | A |
| Install: `claude plugin marketplace add <owner>/<repo>`, then `claude plugin install <plugin>@<marketplace>`; check with `claude plugin validate <path>`. | plugin-marketplaces | A |
| Limit of `instructions` and of a tool description loaded upfront: 4096 characters; through tool search: 16384. Tool output: warning above 10 000 tokens, cut at 25 000 by default. | https://code.claude.com/docs/en/mcp | B |
| A plugin has no documented build step for a Go binary. | plugins-reference | C |

## 3. Registry and MCPB

| Fact | Source | Conf. |
|---|---|---|
| Registry `server.json` needs `name` (`io.github.<user>/<name>`), `description` (at most 100 characters), `version`; each package needs `registryType`, `identifier`, `transport`. Current schema URL in the docs: `https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json`. | https://modelcontextprotocol.io/registry/quickstart, schema | A |
| `registryType` values: `npm`, `pypi`, `nuget`, `cargo`, `oci`, `mcpb`. There is no plain-binary type and no `go`. A Go binary goes as `mcpb` (URL of a `.mcpb` on a GitHub release, `fileSha256` required) or `oci`. | https://modelcontextprotocol.io/registry/package-types | A |
| MCPB: `manifest.json` (`manifest_version` 0.3, `name`, `version`, `description`, `author`, `server`); `server.type: "binary"` with `entry_point` and `compatibility.platforms`; pack with `mcpb pack`. | https://github.com/modelcontextprotocol/mcpb `MANIFEST.md`, `CLI.md` | B |

## 4. Other agents

| Agent | Config | Source | Conf. |
|---|---|---|---|
| Cursor | `.cursor/mcp.json` or `~/.cursor/mcp.json`; `{"mcpServers":{"name":{"command":…,"args":[…],"env":{…}}}}`. Whether `type` is needed in practice is unknown. | https://cursor.com/docs/context/mcp | B |
| VS Code | workspace `.mcp.json` or `~/.copilot/mcp-config.json` with `mcpServers` (`type`, `command`, `args`); `.vscode/mcp.json` with `servers` is the deprecated path. | https://code.visualstudio.com/docs/agents/reference/mcp-configuration | A |
| Gemini CLI | `~/.gemini/settings.json` or `.gemini/settings.json`, key `mcpServers`; no underscores in server names. | https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/mcp-server.md | A |
| Codex CLI | `~/.codex/config.toml` or `.codex/config.toml`, table `[mcp_servers.<name>]` with `command`, `args`, `env`. Codex also has plugins (skills and MCP servers) and marketplaces, but their file format was not read. | https://developers.openai.com/codex/mcp (redirects to learn.chatgpt.com) | B / C for plugins |
| Windsurf | page moved to docs.devin.ai; config path not confirmed | — | C |
| `AGENTS.md` | plain Markdown at the repo root; the closest file to the edited one wins; it does not configure MCP | https://agents.md/ | A |

## Pending

- Format of a Codex plugin marketplace (`/codex/build-plugins`).
- Whether `source: "./"` (the repo root as the plugin) passes `claude plugin validate`; the local CLI (2.1.322) will say.
- Windsurf config path.
- Whether `userConfig` fields can be optional, so that an empty `WEBSCOPE_DB_URL` leaves only `query` off.
