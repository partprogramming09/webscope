# Testing a module like a user and like someone hunting for errors

For webscope 0.5: `query`, `page`, `check` and `code`, plus `Grep` and `Read` for code and `javascript_tool` of Claude in Chrome to run what `page` returns. The order below is the one that found real defects; the old server's version of this file is in the repo history. Patterns seen so far are in [patterns.md](patterns.md); the old tools in [old-to-new-tools.md](old-to-new-tools.md).

A module is done only when step 9 has no pending row.

## Pick the cheapest check that gives enough confidence
Choose the first of these that can show the problem, and go up only when it cannot:
1. **Unit or integration test** of the code that changed (the project's own runner).
2. **API check**: the endpoint and its validation, read with `code map_back {module}` (route, endpoint, controller with line, service, model, table) and `Read`, called or tested without a screen.
3. **E2E in the browser** (`page`): only when the problem is in what the user sees or in a flow that crosses screen, API and database.

To know what exists before choosing: `code map_back {module}` and `code map {module}` (screen routes, calls to the API). Nothing here asks for a subagent: the agent decides whether one is worth it.

Browser order when an E2E is justified:
1. A tab of the app in the MCP group (`tabs_create_mcp`, then navigate) with the session already started. If it shows the login, stop and ask; never type credentials. `WEBSCOPE_APP_URL` must equal the tab's exact origin.
2. `page(load)` once per page load, and run the line it returns with `javascript_tool`. Load it **before** acting: the console is logged only from then on.
3. `page(state)` and `page(read)` for the screen as text; `fill`, `click`, `press` to act; `payloads` for what was sent and answered; `simulate` to force an HTTP error. Each call returns a snippet that `javascript_tool` runs. One `javascript_tool` call may run several `window.__ws` actions in order.

## Completeness rule (applies to every step below)
- **Cover everything the module has.** Every list, tab, form, modal, row action, screen action, filter, sort, page control, import and export is tested or recorded as excluded with the reason. Inventory first (`code map {module} walkthrough`: actions with their `v-if`, permissions, links, fields where `name*` is required, tabs and statuses), test second.
- **Every field, none missing.** Each field gets at least: a valid value, an empty value, a value at its limit, a value past it, a wrong type and a repeated value. The coverage table has one row per field; a field with no row is pending, not skipped.
- **Sub-routes are screens, tested in full.** Every route the module links to gets the same pass as the list. Opening it and listing what it offers is not a test: a sub-route whose writes were not run is pending, never ok.
- **Order: normal use first, then break it.** 1) the module the way a user would use it, with valid data, end to end; 2) only then the hostile pass (section 4) and the data checks (section 5).
- **Fixed order of a module pass, each gate before the next:** (1) routes and screens; (2) inventory of each screen; (3) read-only checks; (4) cleanup proven on one row (create one `ZZ_` row, delete it, see the counts as before); (5) normal use end to end; (6) the hostile pass; (7) data checks against the table; (8) cleanup and coverage table. Fixing the tools never runs in the middle of a module pass: a tool defect found on the way is recorded in the task file and the pass goes on or stops.
- **Cost decides the method.** Before a long pass, estimate its calls (fields × cases). Past about 12 calls of the same shape, run them in one `javascript_tool` call (`fill` takes a whole form). If the full matrix is still too costly, say so and ask which fields or cases to cut; never silently test less.
- **Limits, so a pass never loops.** At most 2 attempts to fix or work around one problem: on the third, stop, record it and go on. A check that already passed on the same code is not run again. Test data a run could not remove is listed (ids) and left, never chased in a loop.

## 1. Before touching the screen
- `code map_back {module}`, `code map {module}`, and `code fields {table}` for each table: columns, how many rows have a value, which backend files name each column.
- `map_back` also lists the models of every shared service the module's endpoints go through (25 tables for `charge-types`, whose own table is one): the module's table is the model under its own folder, so read that one first.
- `code consumers {table, column}` for a column you are about to test: who reads and who writes it (an estimate by text, and it says so).
- Baseline numbers with `query` (with `tenant`): rows of each table, counted by `ESTADO` when the delete is logical. Write them down.
- Never read rows of a real customer: schema and counts only.

## 2. Read it deep
- `page(state)`: URL, title, console, headings, fields, buttons, alerts, errors, dialog, tables. `page(read {selector})` for one part.
- `state` does not mark disabled buttons (a dialog showed `Cancelar Guardar` with `Guardar` disabled) and lists a button with no text only if it has an `aria-label` or `title`: read the rest with `javascript_tool`. Its `Buttons` line keeps the first 30, and the side menu comes first (read in `page.js`), so on a busy screen the page's own buttons may be cut: ask for `read {selector}` of the part you need.
- A dialog's fields are listed in `Fields` after the page's own; `fill` looks in the open dialog first.

## 3. As the user (complete flows, in the browser)
- `fill {fields:{label:value}}` for a whole form (it chooses in Vuetify menus), then `click` the save button. `click` and `press` answer what happened: `Requests:` (method, path, status), `New alerts:`, `New errors:`, `New console errors:`, `URL: a → b`, or `No effect seen`.
- Every row action and every screen action. A row action with no text is clicked by `selector`.
- Test data carries the `ZZ_` prefix so it can be found and removed. A write needs the user's approval, action by action.

## 4. As someone looking for a way in or a way to break it
- Empty form: first read whether the save button is disabled (`javascript_tool`); if it is, fill one required field at a time and read it again to find the rule that enables it (in `charge-types` it was the four fields Tipo, Tipo de Cargo, Base and Operación; Estado and Orden come filled; nothing was written). Pressing a save that is **enabled** is an attempt to write: ask first. Then weak values, mismatched confirmation, over-long values, repeated submit.
- `simulate {match, status, times}` then an action that calls it: the screen must say something (`New alerts:`). A 500 with no notice and the old data still shown is a finding (seen in two modules). `New console errors:` shows what the app logged (`error: Error fetching charge types: …`); `rejected:` lines can come from elsewhere (one `reading 'M_ID'` is not in the app's `src`), so read the code before blaming the app.
- Limits (licences, page size, rate limit), protected records (system rows), a role without the permission: the same flow must be refused, on screen and in the API. Another user's data (the same id with a second token) needs a crafted request, which is outside `page`.

## 5. Data
- A pass with no write approval stops at the baseline: counts by `ESTADO`, `code fields`, `code consumers`, and the screen compared with `query` (in `charge-types`: 3 rows on screen with 1 active and 2 shown as «Eliminado», the same as the table). `check` needs a row written through the screen, so it needs the user's approval.
- After every write read the row back: `check {sent, table, where, map, after?, at?, tenant?}` compares the body the screen sent, the row stored and the answer the screen read, with a map you give. `007` against `7` is a difference; `''` against `NULL` is marked apart.
- `code snap {name, sql, key?}` before and `code snap_diff {name}` after an action shows only what changed. Do it also around **opening a record and saving without touching anything**: it must change nothing (clean in `charge-types`; not clean in quotes, see [patterns.md](patterns.md)).
- A full cycle on one `ZZ_` row, as run in `charge-types` with the user's approval: `snap` → `fill` + `click` save (`POST … 200`, `New alerts: … creado`) → `check` (6 fields, 6 equal, with `ñandú` and `007` intact) → `snap_diff` (one `new` row) → a second `snap`, open the row and save untouched (`PUT … 200`, `check` 6 equal, `snap_diff` «no changes») → delete by screen (`DELETE … 200`; `snap_diff` shows only `estado ACT → BORRADO`). Row actions had no text: the first button of the row was edit and the second delete (read it from the screen, the order is the component's); the delete dialog does not name the record, so confirm that you opened it from the right row.
- Count rows and totals before and after. Clean up through the screen; list leftovers with `query … WHERE column LIKE 'ZZ\_%'`. A screen that cannot delete what the test created is a defect to report.

## 6. Jobs, mail and files
Not covered by webscope 0.5 (no tool reads queues, mailboxes or stored files). Use the project's own tools and write the result in the report: queue supervisor running, failed jobs before and after, the mail receiver (count, recipient, subject), the stored file opened and counted.

## 7. Distrust your own result
- A zero, or the same answer everywhere (all 403, all 200), is a signal about the check, not about the app. Fire the check on a case that must fail; a tool that says «0 problems» without having fired on a known case proves nothing. Examples that were wrong: `null` compared with `''`, a stale token, `fill` saying ok on a menu that put the old value back.
- `No effect seen` is a fact about the click, not a verdict: read whether the button was disabled before calling it a defect.
- A tool that reports success without an effect is a failure.

## 8. Classify what was found
Per finding: can an ordinary user trigger it from the screen? does it damage or expose data? can someone exploit it deliberately? Only what was reproduced and measured is a confirmed error; the rest goes to «not tested».

## 9. Coverage table, then close
One row per route and per row action: tested, excluded with the reason, or pending. The routes come from the code (`code map_back`), not from the screen: bulk import, variations and exports are routes under their own path or verb. Compare the calls `payloads` captured with the endpoints `map_back` lists. Record findings and the table in the project's report, update the task file, and leave the database as it was found.

## 10. Classify every finding by severity
Each finding gets one class, a reproducibility mark and an origin, so the fixes are ordered by harm and not by the order they were found.

| Class | What it means | Examples |
|---|---|---|
| S1 Data lost or corrupted | A value is stored wrong, cut, coerced to another type, or never reaches the database although the screen says it saved; rows are deleted that should not be | a field missing in the request, a column too short, `0` saved as empty, a save with no write, an untouched save that rewrites columns |
| S2 Security | A user can do or read what they should not, or the app leaks | a missing or client-only permission check, another tenant's rows, injection, secrets in a response or a log |
| S3 Function unusable | A feature cannot be used or finished | a button that does nothing, a 500 on valid input, a blank screen, a flow that cannot complete |
| S4 Function degraded | It works but gives a wrong or partial result | wrong filter or sort, a stale list after a save or after a failed request, a missing validation message, a very slow call |
| S5 Interface | Looks or reads wrong, nothing is lost | a hint shown as an error, inconsistent labels, a required field without its mark |

- **Reproducible by a user (yes or no):** yes when ordinary use of the screen triggers it, no when it needs a crafted request or a direct database change. A «no» on S1 or S2 is still serious (the server trusts the form).
- **Origin:** client, request rule, controller or service, database (type, length, constraint), or contract (what was sent, what arrived, what was stored differ). Find it by following one record: the body in `payloads`, the row with `query`, the comparison with `check`.
- **Resisted attempts are recorded too:** an attack from the screen that the server stopped with a 4xx and a clear message goes in the coverage table as `resisted`.
- Row format in the report: `id | module | class | reproducible | origin | where (file:line or route) | evidence | steps`.

## Cost
- The cost is the context re-read on every call, not what a tool returns: fewer calls and a smaller context are the only levers. (Measured on a 12 USD session of 431 calls of the old server.)
- No screenshots to read the screen: `page(state)` gives it as text. Return from `javascript_tool` only the lines you need.
- One call per form (`fill` with several fields), several `window.__ws` actions per `javascript_tool` call, one `check` per request.
- Start a new session when the context passes about 500 k tokens; the task file must already hold what was learned.
