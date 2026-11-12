# ADR 0002: the Claude Code plugin is the repository and builds the binary on first start

**Reader:** whoever changes how webscope is distributed.

**Status:** accepted (2026-11-12).

## Context
webscope is a Go binary. A Claude Code plugin has no documented build step for a Go binary (`docs/research/2026-11-12-mcp-marketplace-and-sdk.md`, section 2). The repository is private and the last tag is `v0.6.5`. The MCP Registry has no plain-binary package type: a Go binary goes as `mcpb` (a `.mcpb` file on a GitHub release with `fileSha256`) or `oci`.

## Options
1. **The plugin is the repository (`source: "./"`) and `bin/webscope-launch` builds the binary the first time.** Needs Go on the installing machine; no release, no tag, works with a private repo.
2. Releases with binaries per platform, packed as `.mcpb`. No Go needed, but it needs a tag, a release and a repo the client can download from.
3. A marketplace `command` source that builds. Claude Code shows the command for approval and runs it at install, update and once per session; the command is limited to 500 ASCII characters.

## Decision
Option 1. The launcher uses `WEBSCOPE_BIN` if it is set, otherwise a binary built from the plugin folder and kept per version in `${CLAUDE_PLUGIN_DATA}`. It writes only to stderr, because stdout is the protocol, and every failure says the next step.

Measured, 3 runs each on one machine: building with an empty build cache and the modules already downloaded took 9.10, 7.63 and 7.74 s; with the module cache empty too (what a new installer has) 14.36, 12.81 and 12.87 s. Claude Code's startup timeout for an MCP server was not verified, so `bin/webscope-launch build` exists to do the first build ahead of time. Installing from a local marketplace, `claude mcp list` showed the plugin's server connected.

## Consequences
- The user needs Go (the version in `go.mod`) and git access to the repository.
- Publishing in the MCP Registry is left out: it needs option 2. The steps then are a tag, a release with the `.mcpb`, a `server.json` with `io.github.<user>/<name>` and the file's SHA-256, `mcp-publisher validate`, and only when the owner asks, `mcp-publisher publish`.
- The version lives in two places (`main.go` and `.claude-plugin/plugin.json`); a test fails if they differ. Users get a new copy only when `version` changes.
