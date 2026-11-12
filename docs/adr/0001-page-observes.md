# ADR 0001: `page` keeps driving the screen; it does not become an observer only

**Reader:** whoever changes the `page` actions, or the skills and guides that use them.

**Status:** accepted (2026-11-11).

## Context
Claude in Chrome now has native tools that type, click, pick, upload and read the network (`docs/research/2026-11-11-claude-in-chrome-tools.md`). If those tools could do the same work, `page` would only need to observe: `payloads {after: N}` and `check`. It could then drop `open`, `fill`, `click`, `press`, `options`, `find`, `read`, `upload` and `batch`, and `page(help)` would get shorter.

The rule set before measuring: an action is removed only if the native tools **can** do it, and doing the task natively does **not** come out worse than doing it with `page`.

## What was measured (2026-11-11)
The task was a create form in a Laravel + Vue (Vuetify) app: four autocompletes, numbers, a note, a supplier file attached before saving, then save. Each run was a new headless Claude Code session (`claude -p --chrome`) on the same machine, with webscope 0.7.18, the same data and the same file. Order: A1, B1, A2, B2, A3, B3.
- **A:** `page` drives the screen (`open`, `fill`, `click`, `upload`).
- **B:** the native tools drive it; `page` only observes (`payloads {after: N}`, `check`).

A run counts as a success if `payloads` holds the create POST with its fields, the upload was captured, and `check` of the POST against the row shows no unexpected difference.

| Run | Calls | Result bytes | Seconds | Cost (USD) | Success |
|---|---|---|---|---|---|
| A1 | 29 | 22 680 | 111 | 1.20 | yes |
| B1 | 49 | 35 930 | 130 | 1.30 | yes |
| A2 | 30 | 25 316 | 113 | 1.17 | yes |
| B2 | 43 | 24 131 | 173 | 1.34 | yes |
| A3 | 33 | 31 303 | 117 | 1.22 | yes |
| B3 | 47 | 33 276 | 132 | 1.31 | yes |
| **A, mean** | **30.7** | **26 433** | **113.7** | **1.19** | 3/3 |
| **B, mean** | **46.3** | **31 112** | **145.0** | **1.32** | 3/3 |
| B against A | +51 % | +18 % | +28 % | +10 % | same |

The ranges do not overlap in calls (A 29–33, B 43–49), seconds (A 111–117, B 130–173) or cost. They do overlap in bytes (B2 is below A2 and A3), so with three runs per arm the «+18 % bytes» is not established.

**Limit of this measurement:** none of the three B runs used `browser_batch`; they made each native call on its own. A discarded pilot B run that did use it (10 times) took 38 calls even with an extra detour. So «+51 % calls» measures native use without grouping. A B arm told to group with `browser_batch` could close the gap for `open`, `fill`, `click`, `press`, `find` and `upload`.

Harness, to repeat it: one new `claude -p --chrome` session per run in the app's project, webscope passed with `--mcp-config`, the upload file in a folder under the temp directory (which `page upload` accepts) also passed with `--add-dir` (which `file_upload` needs), and the prompt asking to attach the file before saving. Clean up from the screen, outside the measured session. Count calls, result bytes and seconds from each session's transcript.

Two pilot runs were discarded before the table. The prompt told the agent to save before attaching the file, and in B the file sat in a folder the session could not read.

## Action by action
| Action | Can the native tools do it? | Decision |
|---|---|---|
| `open` | yes (`navigate`, 1 call) | stays: B is worse overall |
| `fill` | yes (`form_input`, 1 call per field) | stays |
| `click` | yes (`computer`, by ref or coordinate) | stays |
| `press` | yes (`computer key`) | stays |
| `find` | yes (`find` in natural language). `read_page` shows the Vuetify autocompletes as «Open», without their label | stays |
| `options` | partly: the options of some Vuetify selects come out without names in the accessibility tree, and one B run had to pick them with `javascript_tool`. Each field takes 3–4 calls | stays |
| `read` | partly (`get_page_text` reads a list; one table row needed `javascript_tool`) | stays |
| `upload` | yes (`file_upload`), but only for files in a folder shared with the session (`--add-dir`) | stays |
| `batch` | `browser_batch` exists; it stops at the first error | stays |
| `download` | not tested | stays (decided before measuring) |

**Decision: no action is removed.** Every row that says «can» fails the second condition: B comes out worse than A in calls, seconds and cost, as measured. The `browser_batch` limit above is the one thing that could change this. Phase 2 of the plan (0.7.19, removing actions) is not done, and no test is deleted or rewritten.

## What the measurement did confirm
- **The native network tool does not show what a screen sent.** `read_network_requests` returns URL, method and status, with no body. In the exploratory run (one session) it listed only the CORS preflight (`OPTIONS`) of the later table read, and did not show the create POST or the DELETE. In all 6 runs, `payloads` captured the create POST with its fields and the upload. Observing stays a `page` job either way.
- **`payloads {after: N}` and `check` work as an observer for either mode.** All 6 runs, A and B, used them to prove the save.
- **Gap found in `check`:** in all 6 runs, `check` with `after` (the screen's read after saving) failed, because the response of that table read is captured truncated. Candidate for a later version.

## Consequences
- `page(help)`, `tools/list` and the skills stay as they are.
- Using native tools stays allowed and optional: a session may click natively and still observe with `payloads {after: N}`. Nothing blocks or refuses it (the design rule «the tool informs, the model decides»).
- The decision covers one form screen. A screen whose controls are plain HTML could give a different result; repeat the measurement with the harness above before removing anything.

## How to undo it
Nothing changed in the code. To revisit, repeat the measurement on another screen. If B stops being worse (for example with `browser_batch`), write ADR 0002, which replaces this one.
