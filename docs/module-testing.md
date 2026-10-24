> **Written for the old webscope server (`webscope-publico`); copied on 24 October 2026 without changing a word of it.** The tools it names (`e2e_chrome`, `plan`, `options`, `__f.*`, `payload_check`, `run`, …) are not in webscope 0.5 or are called differently: read [old-to-new-tools.md](old-to-new-tools.md) before following a step. What it says about *what to look for* in a module still holds; the commands do not.

# Testing a module like a user and like someone hunting for errors

The order below is the one that found real defects. Every step says what to run and what to record; a module is done only when
step 9 has no pending row.

## Pick the cheapest check that gives enough confidence
Not every change needs the browser. Choose the first of these that can show the problem, and go up only when it cannot:
1. **Unit or integration test** of the code that changed (a script declared in the manifest, such as `go test` or `php artisan test`, run with `run`).
2. **API check**: the endpoint and its validation, read from `map_back` and called or tested without a screen.
3. **E2E in the browser**: only when the problem is in what the user sees or in a flow that crosses screen, API and database.

To know what exists before choosing, read the module's flow in one or two calls: `plan <module>`, `map_back <module>` (route, endpoint, controller:line,
service, model, table) and `map <module>` (screen routes, clicks and API calls); `changed` lists the e2e specs a change touches. Nothing here
asks for a subagent: the agent decides whether one is worth it.

Browser order when an E2E is justified, and the detection that goes with it:
1. **Claude in Chrome**, when the agent has it: `test(action="emit", args=["state"])` returns a short snippet that the agent runs in the user's open tab
   with the `javascript_tool` of Claude in Chrome. It needs no debugging port. The server cannot see whether that tool exists in the client, so the agent
   decides; the snippet refuses to run in a tab outside `project.base_url`.
2. **`test` in a Chrome the user starts with a debugging port and its own profile** (see the README, «Native Chrome Connector»; webscope never starts one: with none running, `tabs` fails and says how). No sessions or cookies of the user's everyday Chrome; sign in once there if the app needs it. The Chrome at `WEBSCOPE_CDP` is the one used. It also opens its own tabs.
3. **Another tool already present** (a Playwright server, the project's own runner) with the cases `plan` lists. Webscope adds no adapter for it.

## Completeness rule (applies to every step below)
- **Cover everything the module has.** Every list, tab, form, modal, row action, screen action, filter, sort, page control and import or export is
  tested, or recorded as excluded with the reason. Inventory first (`map`, `__f.list()`, row icons, `route:list`), test second.
- **Every field, none missing.** The inventory of each form and modal is its full field list (label, kind, required, locked, options). Each field gets, at
  least: a valid value, an empty value, a value at its limit, a value past its limit, a wrong type and a repeated value. The coverage table has one row
  per field; a field with no row is pending, not skipped.
- **Sub-routes are screens, tested in full.** Every route the module links to (the row's name link, an eye or settlement icon, any `:id/...` route, a
  tab that is its own page) gets the same pass as the list: `inventory`, every tab, every add, edit, delete and save, each value read back on screen
  and in the table, and the simulated errors. Opening it and listing what it offers is not a test: a sub-route whose writes were not run is pending in
  the coverage table, never ok. It runs as its own bounded task (one subagent per sub-route), on a record the list pass created.
- **Order: normal use first, then break it.** 1) the whole module the way a user would use it, with valid data, end to end; 2) only then the hostile
  pass (section 4) and the data checks (section 5) to find inconsistencies, errors and problems between screen, request, rule and column.
- **Fixed order of a module pass, each gate before the next:** (1) routes and screens of the module (router, menu links, route files); (2) `inventory` of
  each screen; (3) read-only checks; (4) cleanup proven on one row (create one `ZZ_` row, delete it, see the table as before); (5) normal use end to end;
  (6) `matrix` and the hostile pass; (7) data checks against the table; (8) cleanup and coverage table. Fixing the tools is a separate task that never
  runs in the middle of a module pass: a tool defect found on the way is recorded and the pass goes on or stops.
- **Cost decides the method.** Before a long pass, estimate its calls (fields × cases) and price them with the Cost section. Past about 12 calls of the same
  shape, write one script or use `steps` / `crud` so one call runs the whole matrix, and keep it as an automation (a script of the project, with a test).
  If the full matrix is still too costly, say so and ask which fields or cases to cut; never silently test less.

- **Limits, so a pass never loops.** At most 2 attempts to fix or work around one problem: on the third, stop, record it (task file, with the cause if
  known) and go on to the next part of the module. A check that already passed on the same code is not run again; its result is kept in the project's
  report and re-run only after the code it covers changes. Test data a run could not remove is listed (ids) and left, never chased in a loop.

## 1. Before touching the screen
- `plan <module>`, `map_back <module>`, `options <module> --count`: kind, routes, tables, selects that open empty.
- Every route of the module (`route:list` or `map_back`), and every server file of the old system for the same screen
  (the save procedure, the delete case, side effects such as mails or external accounts).
- Baseline numbers: rows of each table, licences or limits, failed jobs, messages in the mail receiver. Write them down.

## 2. Read it deep
- `e2e_chrome --map <module>`: lists, tabs, first record, row menu and row icons. Every icon is an action to test.
- `__f.list()` marks `[locked]` fields; `__f.locked()` lists them. A locked field cannot be edited on screen: the test must say so.

## 3. As the user (complete flows, in the browser)
- `__f.crud({...})` for create, edit and delete in one call; `__f.fill`, `__f.read`, `__f.press` for anything else.
- Every row action (duplicate, activate, reset, assign) and every screen action (generate, export, notify).
- Test data carries the `ZZ_` prefix so it can be found and removed.

## 4. As someone looking for a way in or a way to break it
- Empty form, weak values, mismatched confirmation, over-long values, repeated submit.
- Limits: licences, page size, rate limit. Fill them up and ask for one more.
- Protected records (the support user, system rows): try to edit and delete them.
- Another user's data: call the same id with a second token; expect 404 or 403, never 200.
- A role without the permission: the same flow must be refused, on screen and in the API.

## 5. Data
- After every write read the row back: every column, not only the one that changed. Compare with what the old system wrote
  (columns, enum values, delete marker, timestamps, side effects).
- Count rows and totals before and after. Clean up through the screen; list leftovers with `cleanup_test_rows` (read-only). A screen that cannot delete what the test created is a defect to report.

## 6. Jobs, mail and files
- Queue supervisor running; failed jobs before and after; the mail receiver (count, recipient, subject, content without printing secrets);
  the stored file (open it and count rows and headers).

## 7. Distrust your own result
- A zero, or the same answer everywhere (all 403, all 200), is a signal about the check, not about the app. Fire the check on a case
  that must fail. Examples that were wrong: a class check without the autoload, `null` compared with `''`, a stale token.
- A tool that reports success without an effect (a button that did nothing) is a failure.

## 8. Classify what was found
Per finding: can an ordinary user trigger it from the screen? does it damage or expose data? can someone exploit it deliberately?
Blocks launch / high / medium / low. Only what was reproduced and measured is a confirmed error; the rest goes to "not tested".

## Automated pass (one command per module)
`python3 -I packs/laravel/module_pass.py <Area/Module> <front route>` runs the order above for one module from the project root: it opens the module in a tab of its own
(shared session, closed at the end), reads the inventory, finds the create button, runs `matrix`, crosses the calls the page made with the module's routes
(`map_back --seen`) and runs the field chain (`chain_run.py`: front, rule, model, migration, live column, stored rows). It writes one JSON per module;
`python3 -I packs/laravel/chain_docs.py <site folder> <json>...` turns the JSONs into a VitePress site, one page per module, with the code that causes each flag, why it
happens and the data it hurts. Spec and the ten checks: `docs/chain/SPEC.md`. Measured on Acme's services module: 36 s, 28 flags, 5 of 24 routes exercised by the list screen.

## Cost (measured on a 12 USD session of 431 calls)
- The cost is the context re-read on every call (about 290 k tokens each, 92 M in total), not what a tool returns. Fewer calls and a
  smaller context are the only levers.
- No screenshots to read the screen: `__f.state()` gives route, title, dialog buttons, rows, notices and errors in text. 21 screenshots
  added 407 k characters that every later call re-read.
- Loops of many small calls (browser flows, API probes, SQL checks) go to a subagent that returns the conclusion only. The session's own
  estimate for this work: Bash 37.5 M re-read here against about 11.6 M in a subagent, webscope calls 22.4 M against 4.3 M, browser
  scripts 16.2 M against 2.5 M. One subagent per module phase, at most 40 calls each.
- One call per form: `__f.crud` for the cycle, `__f.fill` with several fields, `__f.emptySubmit` for validation. One script for all the API
  probes of a module instead of one command per probe.
- Start a new session when the context passes about 500 k tokens; the task file must already hold what was learned.

## 9. Coverage table, then close
One row per route and per row action: tested, excluded with the reason, or pending. The routes come from the code, not from the screen: a module's other
functions (bulk import, variations, exports) are routes of its route file under their own path or verb. After the run, save the test tool's `network` answer to a
file and run `map_back <module> --seen <file>`: every route of the module comes back `[tested]` or `[pending]`, plus the calls the front makes to a route the
backend does not have. Record findings and the table in the project's
report, update the task file, and leave the database as it was found.

## 10. Classify every finding by severity
Each finding gets one class, a reproducibility mark and an origin, so the fixes are ordered by harm and not by the order they were found.

| Class | What it means | Examples |
|---|---|---|
| S1 Data lost or corrupted | A value is stored wrong, cut, coerced to another type, or never reaches the database although the screen says it saved; rows are deleted that should not be | payload field missing in the request, column too short, `0` saved as empty, save with a success notice and no write, delete of another row |
| S2 Security | A user can do or read what they should not, or the app leaks | missing or client-only permission check, another tenant's rows, SQL or script injection, secrets in a response or a log, a server rule that only the form enforces |
| S3 Function unusable | A feature cannot be used or finished | a button that does nothing, a 500 on valid input, a blank screen, a flow that cannot complete |
| S4 Function degraded | It works but gives a wrong or partial result | wrong filter or sort, page off by one, stale list after a save, a missing validation message, a very slow call |
| S5 Interface | Looks or reads wrong, nothing is lost | a hint shown as an error, inconsistent labels, a required field without its mark |

- **Reproducible by a user (yes or no):** yes when ordinary use of the screen triggers it, no when it needs a crafted request or a direct database change. A «no» on
  S1 or S2 is still serious (the server trusts the form); a «yes» raises the finding one place in the order inside its class.
- **Origin:** client, request rule, controller or service, database (type, length, constraint), or contract (what was sent, what arrived, what was stored differ).
  Find it by following one record: the request body in the capture, the validated data in the service, the row in the table (`payload_check`, `snap`, `sql`, `audit_types`).
- **Resisted attempts are recorded too:** an attack from the screen that the app stopped (the server rejected it with 4xx and a clear message) goes in the coverage table as
  `resisted`; that is the evidence the module is safe enough from the front.
- Row format in the report: `id | module | class | reproducible | origin | where (file:line or route) | evidence | steps`.
