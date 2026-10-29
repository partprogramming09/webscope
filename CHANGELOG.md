# Changelog

One entry per version, newest first. Tool names are `query`, `page`, `check` and `code`. Each change says what it is, what was seen that made it necessary, how it was checked and what was not checked. A change in what a tool prints is listed as a change, because a client may read it.

## 0.5.28 — 2026-10-29

**New: `page parity {req_a, req_b, map?}`. Same button on the old screen and the new one: do they send and receive the same fields?**

Press the same action on both screens, then name the two requests: each spec is a method and a part of the address (`POST save.php`, `POST /api/quotes`), and the latest capture that matches is used. The answer has the two request lines, `status: 200 → 201` only if it differs, and the field names that only one side sent (`sent only in a`, `sent only in b`) or received (`got only in a`, `got only in b`). **Never a value**: a test with a value in each body checks that it does not come out.

- JSON bodies: nested names with a dot (`data.status`), a list by its first element (`lines[].qty`), four levels deep, 200 names at most.
- Form bodies (`DESCRIPCION=x&OFICINA=MIA`), which is what the old jQuery pages send.
- A body that was cut by the capture limit says `cut` and is not compared as if it were whole; one that is neither JSON nor a form says `not JSON or form`.
- `map {name in a: name in b}` renames a's names first, because the old pages name fields their own way (`DESCRIPCION` is `notes`). Without a map, every renamed field shows on both sides, which is true but noisy; the skill says to build the map from `code fields` and the screen's field list.
- If no capture matches, it names the spec it could not find and says to run the action and look at `payloads`.

`parity` with `a` and `b` still compares two saved screen shapes (0.5.27); with `req_a` and `req_b` it compares requests.

What was seen that made it necessary: `check` compares what was sent with what was saved, but a migration asks something before that: does the new screen send what the old one sent? Until now the agent read two `payloads` and compared by eye.

How it was checked:
- Two tests written first and failing (`parity needs a and b`): a form request against a JSON one with and without `map` (status, names on each side, no value leaks, a mapped name does not appear); and the failures (no capture matches, `req_b` missing, a cut body called `cut`).
- `go test -race ./...` passes (118 tests).
- Not checked: on the real legacy and modern «Buscar», where the names will not all map; a legacy page that answers HTML (it says `not JSON or form`, but I did not see it on a real answer); a body that holds a JSON string inside a form field (it is one name, not opened).

## 0.5.27 — 2026-10-29

**New: `page shape {name}` and `page parity {a, b}`. Put the old screen and the new one side by side and read only what is different.**

`shape {name}` (a snippet, like `state`) saves what the screen in front of you offers, under a name, in the local listener and in memory: field labels with their kind (`text`, `select-one`, `checkbox`, `editor`), buttons, table columns, tabs and headings. No values. The side menu and the top bar are left out (and tabs are listed as tabs, not as buttons), so a menu that differs between the two apps does not drown the result. It answers with the counts and the next step.

`parity {a, b}` is answered by the server, with no snippet. It compares two saved shapes by name, ignoring case and spaces, and prints:
- `only in a` and `only in b`, one line per `field`, `button`, `column`, `tab` or `heading`;
- `changes`: a field that kept its label and changed kind (`field Estado: select-one → text`);
- `no differences …` when there are none, with the reminder that it compares names, not behaviour.

It lives in `page` and not in `code` (the plan said `code parity`): the shapes are kept by the listener that `page` already runs, and `code` would need a new link to it. Up to 20 shapes, each up to 64 KiB; the 21st name is refused with 409. They are lost when the server restarts.

What was seen that made it necessary: the `screen-audit` skill told the agent to `state` both screens and compare the two lists by eye, which costs two long answers and misses a field in a list of thirty. The comparison now costs one short answer.

How it was checked:
- Four tests written first and failing (`404` on `/shape`, `unknown action "parity"`): the diff with case differences, a kind change and the same-on-both-sides names that must not appear; the two missing-shape errors; the refusals (not JSON, no name, no token); and a real Chrome test where the shape travels from another origin to the listener and holds `Estado` as `select-one` and the buttons `Guardar` and `Cerrar`.
- `go test -race ./...` passes (116 tests); `tools/list` is unchanged in size apart from the word `parity`.
- Not checked: on the real legacy and modern quotes list (needs the helper loaded in your Chrome and the two screens opened with the same filters); a screen whose fields are drawn after a delay (the shape is a photo of the moment); two screens that name the same field in two languages (they show as one `only in` on each side, and the skill says to read them as a pair).

## 0.5.26 — 2026-10-29

**New: `code flows {module}`.** One answer with the evidence for the user stories of a front module: `## screens` (what `map {module, walkthrough}` prints: routes, actions with their `v-if`, permissions, links, fields, statuses) and `## endpoints` (what `map_back {module}` prints: endpoint, controller with `file:line`, the front call, services, models and tables). It drops the duplicated headers and routes of the second part. Without `WEBSCOPE_BACKEND` it gives the screens and says `not available: WEBSCOPE_BACKEND is not set`. A module that does not exist gets the same «no module contains» text as `map`. It prints no stories: it is evidence, and says so in its first line.

**New skill `.claude/skills/module-flows/SKILL.md`.** How to write the stories from that evidence (one per action that changes something or leads to another screen, with the endpoint, controller and table as evidence, and the unknowns marked), how to check one flow in the real screen with `page` and `check`, and that a person confirms each story before tests are written from it. Not tried with a real agent yet.

**Also:** `ROADMAP.md` has a new section, «Migraciones: paridad, flujos y pruebas», with the versions 0.5.26 to 0.5.31 and three open questions about the legacy system.

How it was checked:
- Two tests on the fixture of the `map` tests, written first and failing with `unknown action "flows"`: the join (sections, an endpoint, a table, no secret of a `.env`, `node_modules` or a symlinked folder) and the missing backend. `go test -race ./...` passes (112 tests). `tools/list` is 1526 bytes.
- Not checked: against the real application (the registered server has to be rebuilt and reconnected with `/mcp` first); the size of the answer on a large module (it is cut at 8000 bytes, so the end of `## endpoints` can be missing); that the stories the skill produces are good.

## 0.5.25 — 2026-10-29

**New: `page audit {selector?}`.** Lists what the page itself shows wrong, with no reference screen to compare to:
- `unrendered`: text left as written (`{{ x }}`, `undefined`, `null`, `NaN`, `[object Object]`, `$t(`).
- `covered`: a control whose centre is under another element. Not reported: a modal over the page, the wrapper of the control itself (Vuetify draws one over its native input) and a control scrolled out of a container that clips.
- `no label`: fields that nothing names, with their `name`, `id` and a short CSS path so they can be found.
- `same label`: two fields that answer to the same name (a filter and a form field, say); `fill` takes the first.
- `no name`: buttons and links with no text, `aria-label` or title.

Without findings it says so. `selector` limits it to a part of the screen, such as a dialog. It is a hint, not a verdict: it cannot say what should be there, which is the job of the new skill below.

How it was checked:
- A Chrome test page with each situation, plus a clean part, a scrolled-out field and a field under its own wrapper; each case failed before its fix.
- Legacy quotes list: 3 `select` without label (`lstServicioBC`, `lstIncotermBC`, `sentido`), `Estado` twice and 14 unnamed buttons and links.
- Modern quotes dialog «Nueva Cotización»: first `covered (2)` (Incoterm and Moneda), a false positive: the dialog's scrolling area ends at y=837 and those fields sit at y=924, where the backdrop shows. After the fix it says `No findings`.
- Not checked: the legacy with the final build, and whether the 23 unnamed icon buttons of the modern list are useful findings or noise.

**`page state` and `fill`: a field with an empty `<label>` is named by its `aria-label`, `placeholder`, `name` or `id`.** Before, a field whose first `<label>` had no text printed `(no label)` even with a placeholder. On the legacy quotes list `state` showed 10 such fields (7 text, 3 `select`); now it shows none: the text fields take their placeholder and the selects their `name`. What `state` prints for these fields changes, and `fill` can now find them by that name.

**New skill `.claude/skills/screen-audit/SKILL.md`.** How to review one screen with `state` and `audit`, how to look for fields that are missing or should not be there (against `code fields`, `code map_back` and the other version of the screen), how to compare an old and a new screen, and how to report. It keeps the read-only rules and names no customer. Not tried with a real agent yet.

## 0.5.24 — 2026-10-28

**`page` sees the requests of pages that replaced `window.XMLHttpRequest`, as the legacy pages of the app do.**
- What was seen: on the legacy quotes list, «Buscar» made 4 `xmlhttprequest` requests in the browser, while `click` said `No effect seen: no request` and `payloads` showed 0.
- Cause: the page replaces the constructor; its instances keep the native prototype and have their own `open`, so patching `XMLHttpRequest.prototype` was never reached.
- Now the helper patches the prototype the instances really have and, when the constructor is not native, hooks each instance's own `open` and `send`. A send that reaches both hooks is reported once.
- Result: `click` reports `POST …/grid_ppal.php 200 | …/cotiza_frm.php 200 | …/preliquidacion.php 200 | …/cotiza_listas.php 200`, and `payloads` shows what was sent (form-encoded) and the JSON that came back. `WEBSCOPE_APP_URL` must be the origin of the legacy tab.
- Checked with a Chrome test page that replaces the constructor the same way; it failed before (`0 requests captured`) and passes now.
- Not checked: writes on the legacy (`Nuevo` then `Grabar`), forms that send the whole page instead of an XHR, and requests made from an iframe.

**`page fill` on a native `<select>` lists the options when the one asked for is missing.** Before: `no option 25`. Now: `no option "25" among 4 seen: 5 | 10 | 20 | 50` (the first five), the same shape the Vuetify menus already had.

**Tests only:** the Chrome fixtures wait for the helper script before each step. One run on the Ubuntu runner failed with `webscope is not loaded in this page`; the same message appears when `page.js` is served 1.5 s late, and the wait removes it. The package time on that runner varied from 4.9 s to 38.4 s between runs with almost the same code.

## 0.5.23 — 2026-10-28

**`page click`: when nothing matches, the error lists the dialog's buttons first.**
- What was seen: deleting a test quote, `click {text:"Sí, Eliminar"}` answered `nothing to click` and listed the side menu. The button reads «Si, Eliminar», without the accent, and the typed text had one; the error showed the first 12 buttons in page order, which are the side menu's, so the real text never appeared. (The first reading, that the helper could not see the dialog, was wrong.)
- Now the list is ordered like `state`: the dialog's buttons, then the page's, then the side menu's.

**`page click {text}`: when no button, link or menu item has the text, it clicks the plain element (a `span` or `div` with no child elements) whose text is exactly that.** Two or more matches still fail and name them. No real screen needed it yet; it is checked only on a Chrome test page.

**`page fill` writes into a rich-text editor (`contenteditable`).** It selects the content and inserts the text, as typing would. The field is found by its `aria-label` or id, or as `editor` when it has neither: the real editor of a quote's terms (ProseMirror) has no label, so `fill {"editor": "…"}` is how to reach it (the first one when there are several).
- Checked on the real editor of a quote's terms: `fill` answered `editor: ok` and the editor held `<p>…</p>`; the edit was cancelled and no `PUT` or `POST` was captured.
- Not checked: that the app's own model takes the text, which needs pressing «Guardar» on a row.

**`page state` lists such editors under `Fields:`** as `[editor] <label> = "<text>"` (`editor` when there is no label). Before they were not listed. The help says so.

## 0.5.22 — 2026-10-28

**The code is split into `internal/` packages; `main.go` (95 lines) only registers the four tools.**
- `db`: the connection, the statement guard and the tenant lookup; `page`: the snippets, the embedded `page.js` and the listener that keeps the captures; `check`: the comparison, which reads the captures through a small interface instead of reaching into the page tool; `code`: the dispatch, `fields`, `consumers`, `snap`, and the two files tied to a stack (`laravel.go`, `vue.go`); `fsx`: the file walk that never follows a link; `input`: the shape of a call and the cap of an answer.
- Nothing a tool prints changed. `tools/list` is 1443 bytes before and after (`initialize` 1156), and the tests moved next to the code they cover (107 top-level tests).
- Checked on the real server after the page and the code steps: `query` with a tenant (59 rows), `code fields` (42 columns), `code map` (2 routes), `check` without captures (its error) and `snap_diff` (no changes).
- Not checked at that moment: a full round trip in the browser. It was done afterwards: create a test quote on the screen, `check`, `snap_diff`, delete it.

**New `docs/usage.md`:** the contract of each tool, what it guarantees and what it cannot do, and the recommended order of use (`code fields`, `code map`, `code snap`, `page`, `check`, `code snap_diff`).

## 0.5.21 — 2026-10-28
- `page` accepts its arguments at the top level of the call.
- `page` `fill` takes `{label: value}` without `fields`; its error shows an example and the help says how.
- The help says to repeat `load` after navigating, reloading or `/mcp`.

## 0.5.20 — 2026-10-28
- `query`: an unknown column (1054) or table (1146) error adds the real columns of the tables in `FROM` and `JOIN` (at most 40, the closest names first).

## 0.5.19 — 2026-10-28
- `page`: `state`, `click` and `press` warn when the tab's origin is not `WEBSCOPE_APP_URL`.
- `page`: a request nobody answered shows as `0 (no answer)`.

## 0.5.18 — 2026-10-28
- `query`: a missing table asked with `tenant` says that a central table is read without it.

## 0.5.17 — 2026-10-28
- `page(state)`: the Vuetify pager counts as the page's buttons, not the side menu.

## 0.5.16 — 2026-10-27
- `page(state)`: disabled buttons are marked `(disabled)`; the dialog's buttons come first and the side menu's last.
- Change in output: the `state` line `Buttons:` is now `Buttons (dialog first, side menu last):`.

## 0.5.15 — 2026-10-27
- `page(state)` shows the console lines logged since `load`; `click` and `press` report the new ones (`console.error`, `console.warn`, uncaught errors, rejected promises; no query strings).

## 0.5.14 — 2026-10-27
- Change in output: the requests that `click` and `press` report carry no query string (it can hold a token). `payloads` still shows the full address.

## 0.5.13 — 2026-10-27
- `page`: `click` and `press` say what happened: requests with status, new alerts and errors, URL change, or `No effect seen`.
- Change in behavior: both now wait at least 550 ms for the answer (about 2 s more when a request is pending).

## 0.5.12 — 2026-10-27
- `page(help)` says which tab it needs, how to open it, and to stop at the login.

## 0.5.11 — 2026-10-27
- `query`: an unknown column or table error says how to see the real columns or tables.

## 0.5.10 — 2026-10-27
- `code consumers` judges the line of each use (an assignment or an array key next to a write call), not only the 3 lines around.
- Change in output: a comparison, a filter or a value read next to a write call is now `READS`, so the `WRITES` counts of `consumers` and of `fields` can drop; the `BACKEND` header text changed.

## 0.5.9 — 2026-10-27
- `page` `fill`: an autocomplete is retried with the first word of the text when the server found nothing for the whole text.

## 0.5.8 — 2026-10-27
- `page` `fill`: a Vuetify menu selection is no longer read twice, so a menu field chosen with the whole text of its option is reported `ok`.

## 0.5.7 — 2026-10-27 (no tag)
- `check` lists a field the read does not name as `unread~` instead of `different` when sent and stored agree.
- Change in output: the summary line adds `, N unread~` and a line naming those fields; they no longer count as `equal`.

## 0.5.6 — 2026-10-27
- `code` accepts its arguments at the top level of the call too.

## 0.5.5 — 2026-10-24
- `page(payloads)` says when a request from another origin was refused.

## 0.5.4 — 2026-10-24
- `code map` and `map_back` say what a query can be when nothing matches.
