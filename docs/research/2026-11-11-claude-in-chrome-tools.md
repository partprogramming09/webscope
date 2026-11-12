# Claude in Chrome: native tools that could replace `page` actions

**Reader:** whoever decides which `page` actions to keep (ADR `0001-page-observes`).

Question: which native Claude in Chrome tools type into fields, pick an option, upload a file, press a key and read the network, and whether the network tool gives request bodies.

| Fact | Source | Date read | Confidence |
|---|---|---|---|
| The official pages do not list the tool names; the list for a given install is shown by `/mcp` → `claude-in-chrome` → «View tools». | [Use Claude Code with Chrome](https://code.claude.com/docs/en/chrome) | 2026-11-11 | High |
| The only tool names the page mentions are `tabs_context_mcp` and `browser_batch`. | same | 2026-11-11 | High |
| Claude can attach files from the machine to upload fields: up to 10 MB per upload; files with several hard links and credential-like files (`.env`, `.pem`, `.key`, `.ssh/…`) are refused. | same | 2026-11-11 | High |
| Claude can read console output, errors, network requests and DOM state. Whether the network tool returns bodies, or only method, address and status, is not stated. | [Get started with Claude in Chrome](https://support.claude.com/en/articles/12012173-get-started-with-claude-in-chrome) (shows «updated over a week ago», no absolute date) | 2026-11-11 | Medium |
| Typing and clicking are stated; picking an option in a select, pressing a key and running JavaScript are not stated on either page. | both pages | 2026-11-11 | Medium |
| A different product, the API's browser-use toolset, documents `form_input`, `file_upload`, `read_network` (requests since the last read, only from when it attached) and `javascript_exec`. It is not proven that Claude in Chrome exposes the same tools. | [Browser use tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/browser-use-tool) (search excerpt, page not read in full) | 2026-11-11 | Low |

## Pending
- The real tool list of the installed extension (`/mcp` → View tools), saved with the extension version.
- Whether the network tool returns the body of a POST: one POST on the real app, then one read of the network, compared with `page(payloads)`. Until this is known, `payloads` is the only place that shows what a screen sent.
