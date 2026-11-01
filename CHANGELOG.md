# Changelog

One entry per version, newest first. Tool names are `query`, `page`, `check` and `code`. Each change says what it is, what was seen that made it necessary, how it was checked and what was not checked. A change in what a tool prints is listed as a change, because a client may read it.

## 0.7.7 — 2026-11-01

**`page download` says why no file came: partial file, newest file, last file seen, and what Chrome is set to.**

What was seen that made it necessary: a colleague's report of 7 failures of `page download` in a day, all `no file appeared in ~/Downloads within 10-20 s`, with nothing in the answer to tell a click that never ran from a download Chrome was waiting to place.

Now the failure keeps its first sentence and adds what can be known: a `.crdownload` in the folder (Chrome was still writing: wait longer); the newest file there and its age in minutes; the last file this session saw appear (from `page(download, {report:true})`); and, from the `Preferences` file of each Chrome or Chromium profile on this machine, a profile that asks where to save each file (`download.prompt_for_download`) or whose `download.default_directory` is not the watched folder. Only those two keys are read from `Preferences`; nothing else of it is read or printed. The folder itself was already named. A failure with none of these facts reads as before.

How it was checked: a new test that fails without the change (a partial file, an older file, a profile with `prompt_for_download` and another folder, and a secret in another key that must not appear); the suite passes with `-race`.

What was not checked: Chrome's own dialog on the real app (see the entry added after the last version of this series); profiles outside the standard folders on macOS and Linux; Windows.

## 0.7.6 — 2026-11-01

**`find` reads a text written as /A|B/ as a regular expression and says when a `|` was searched as plain text.**

What was seen that made it necessary: a colleague's report: `find` with `A|B` treated the `|` as plain text and answered `No field, button or text matches`, with no hint that a regex was meant.

Now a `text` written between slashes, `/A|B/`, is read as a regular expression (case-insensitive), on purpose; `regex` works as before. A `text` that holds a `|`, is not between slashes and finds nothing ends with `«text» is searched as written, with the | inside it; for alternatives use find {"regex":"A|B"} or write the text as /A|B/`. A bad expression written between slashes is `Failed: bad regex /…/`. A text that is not between slashes behaves as before.

How it was checked: the Chrome test of `find` got three cases that fail without the change (`/^guard|^cancel/` finds both buttons; `Guardar|Cancelar` finds nothing and says why; `/(/` is a bad regex); the suite passes with `-race`.

What was not checked: on the real app (see the entry added after the last version of this series).

## 0.7.5 — 2026-11-01

**`fill` ends with the state of the form: whether the main button is enabled and which fields show an error.**

What was seen that made it necessary: a colleague's report of the parity of two Acme modules: `fill` answered `ok` for every field while the form stayed invalid and the save button disabled, so a fill that did not enable the form read as done.

Now the answer of `fill` ends with a line such as `Form: «Guardar» is disabled; fields with an error: Correo (Correo no válido).`. The button is the first visible one of the open dialog (or of the page, outside the side menu and the top bar) whose text says save, create, accept, send, update or confirm (Spanish or English). A field shows an error when it has `aria-invalid="true"` or sits in a Vuetify `v-input--error`, `is-invalid` or `has-error` box; its message is the error text of that box, 60 characters at most. With the button disabled and no field in error, the line says that a required or invalid field may be missing and points to `page(state)`. With no such button and no field in error the line is not added. It reads the page, writes nothing and sends nothing.

How it was checked: a new Chrome test that fails without the change (a dialog with a field in error and a disabled `Guardar` that a script enables when the email has an `@`: the first `fill` says disabled and names `Correo (Correo no válido)`, the second says enabled); 211 tests pass with `-race`.

On the real app (`demo.localhost`, users screen, «Nuevo Usuario» open, nothing saved): `fill {Nombre: ZZ_prueba, Correo: no-es-correo}` ended with `Form: «Guardar» is disabled; fields with an error: Correo.`; the error text of the field was not shown there (the app draws it somewhere the helper does not read, or later).

What was not checked: other libraries' markup for an invalid field (only the classes above); a main button whose text is not in the list above (the line is then not added unless a field is in error).

## 0.7.4 — 2026-10-31

**Any `page` action loads the helper again by itself, the interceptor warning tells a wrapper from a stand-in, the refusal of `payloads` says which origin counts, and a cut `state` keeps the dialog.**

What was seen that made it necessary: a colleague's report of the parity of two Acme modules (441 calls and 45 failures in a day; the server log does not see what runs inside the browser). Four items, in her order.

1. Lost helper. After a reload or a navigation the snippet answered `webscope is not loaded in this page` and `page(load)` had to be run about 8 times in a short session. Now every snippet carries the address of the helper: with `window.__ws` present it runs as before; without it, it adds the script, waits for it and runs the action. If the script cannot load (the page's security policy, the listener down) the answer is `Failed: the helper could not load in this page … Next: page(load)`. The snippets are about 200 characters longer. Without `WEBSCOPE_APP_URL` the old failure stays. After `/mcp` (new port and token) a page that still holds the old helper needs `page(load)`: a helper that is there is not replaced.
2. Interceptor warning. The helper only compared `fetch` with its own function, so any later wrapper (a library, an analytics script) read as «what follows is not a real click». Now, when `fetch` was replaced, the helper calls the listener's address through it: if that reaches the helper's `fetch`, the replacement is a wrapper and the answer says `Note: fetch was wrapped by another script … the wrapper calls the helper (checked), so the requests listed are the ones the app sent; whether it changes the answers the app receives is not checked`. If it does not (a stand-in such as `async()=>new Response('{}')`), the old `Interceptor:` warning stays, and so does the downgrade of `verify` to INFERIDO. A wrapper alone does not downgrade `verify`. A replaced `send` or anchor `click` keeps the warning. New route `/ping` of the listener.
3. `payloads` refusal. The report said requests to the API (`:8001`) from an app on `:5173` were refused. Read in the code, the requests come from the helper inside the tab, so their origin is the tab's; the API's origin never reaches the listener, and a test with an API on another origin shows it is captured. The refusal is about the tab's address (`localhost:5173` against `demo.localhost:5173`). The message now says that only the tab's origin counts and that `state` says which address the tab has. The origin itself is still not repeated in the answer (an existing test requires it).
4. `state` cut. The notice said `raise max`, and the whole answer then went through `javascript_tool`'s limit. The notice now says that a longer answer is kept whole for `page(result)`. The order of `state` changed: URL, title, dialog, alerts, errors, console, buttons, headings, then fields (those in a dialog first), tables and footer. Before, `Dialog:` came near the end and was the first thing a cut dropped. Also, `state` with a `max` too small for one long line (the console line, the side menu) used to stop there and drop every line after it, even short ones; now a line that does not fit is skipped and the next ones are tried (seen on the real app: `max:700` on the users screen showed only URL and title).

How it was checked: three new Chrome tests that fail without the change (an action on a page with no helper; a wrapper that calls the helper gives the note and not the warning, and `verify` is not downgraded; a `state` cut at 600 characters on a page with 60 fields and a dialog keeps the dialog, its field and its button, and names `page(result)`), a test that the refusal says only the tab counts, and one that an API on another origin is captured (it passes without the change: it records that such a request is not refused). The suite passes with `-race`.

On the real app (`demo.localhost`, session open): a page with no helper loaded it from `state` and `open` (twice, after a login and after a reload); `payloads` showed `GET demo.localhost:8001/api/settings 200` with only `:5173` and `:8088` in `WEBSCOPE_APP_URL`; no false interceptor warning on the dashboard or the users screen; with «Nuevo Usuario» open, `state {max:800}` kept the dialog, its fields first and the page's after.

What was not checked: the note for a real `fetch` wrapper (none was found in the app) and the legacy; a page whose security policy blocks `127.0.0.1`; a wrapper that calls the helper but changes the answers (the note says so); that the colleague's refusal was a tab on another address, which comes from the code and `patterns.md`, not from her session.

## 0.7.3 — 2026-10-31

**`simulate` takes a body, `fill` takes a CSS selector, and `code` calls with no tenant use the only tenant there is.**

What was seen that made it necessary: the colleague's list still had three open items. `simulate` could set only a status, so an error screen that shows the server's validation text could not be tested. `fill` had no way to name a field with no label, name or id (legacy inputs such as `data-variacion`). `code rows` with no tenant failed on the real project with `Table '…' doesn't exist; the call named no tenant`, although the central database holds one tenant.

What changed. `simulate {match, status, body?}`: `body` is a text or an object (sent as JSON), 2000 characters at most, for a simulated `fetch` and a simulated XMLHttpRequest; without it the short default message stays. `fill {"css:<selector>": value}` fills the first visible field that the selector matches (`not found` when none does, or the selector is not valid). When a `code` call names no tenant, fails because a table is missing (MySQL 1146 or SQLite `no such table`) and the central database (Hyn Tenancy) has exactly one tenant, the call runs again with that tenant and the answer starts with `(tenant X: the only one in the central database, used because the call named none)`; with two tenants or more nothing is guessed and the error is the same as before. A tenant that is named is always used as given. The new arguments are optional.

How it was checked: three new tests that fail without the change (the Chrome test of `simulate` bodies, with fetch, XMLHttpRequest, the default and a body over the limit; the `css:` cases added to the Chrome test of `find`; the one-tenant and two-tenant tests of `rows`); 204 tests pass with `-race`, `go vet` and `gofmt`. On the real app (`demo.localhost`): `code rows {table: app_tipo_tarifario, a: 1, b: 2}` with no tenant answered with the tenant line and the comparison; `fill {"css:#input-56": "ZZ_css"}` put the text in the search field and the next list request carried `search=ZZ_css`; `simulate` of `/api/sales-tariffs` with status 422 and a body made the screen show that body's message in its alert, and `payloads` showed `422 … {"message":"Prueba webscope cuerpo",…}`.

What was not checked: `css:` on the legacy; `simulate` with a body over a request that the app sends through something other than `fetch` or XMLHttpRequest; the tenant default with a central database that uses its own `WEBSCOPE_TENANT_QUERY`.

## 0.7.2 — 2026-10-31

**`page click` tells a repeated confirmation apart from a click that did nothing, and says when the button it pressed is disabled.**

What was seen that made it necessary: the colleague's report that the first click on «Si, Eliminar» did not delete. Reproduced on the real app 6 of 6 times (tariff configurations, rows `ZZ_` created and deleted through the screen): the first click sends no request, the second sends `DELETE`. Cause, read in the front (`TariffConfigurationList.vue`): the data table opens its own delete modal and, on confirm, emits `deleteItem`, whose handler opens a second `ConfirmationModal` with the same question; a `MutationObserver` showed the first dialog closing and another opening. It is not a failure of the click, the delay between calls, or the events sent (a full pointer sequence behaved the same). `click` answered «The screen text changed; no request, alert or URL change», which reads as a click that did not work.

Now, when the button pressed sat in a dialog, is gone from the page after the click, no request was sent and a dialog with the same first line (its question) is open, the answer says: the dialog closed and one with the same question is open again, a repeated confirmation, it did not fail. Two first versions of this check failed on the real app and were corrected: one needed the answer to hold nothing else (an unrelated console error broke it) and one compared the whole text (the two modals differ: the first has an extra message). A click on a disabled button now ends with «(it is disabled: the app ignores a click on it)».

How it was checked: the Chrome test of a dialog that is replaced by another with the same question (the first has an extra paragraph and an unrelated rejected promise happens during the click) fails without the change and passes; a click on a disabled button and one that changes nothing keep their answers; 202 tests pass with `-race`. On the real app: the first «Si, Eliminar» printed the repeated-confirmation text (with an `M_ID` console error in the same answer), the second sent `DELETE …/608`. `check POST /api/configs/tariff-types` against the created row (id 598) gave 4 of 4 fields equal; all rows created for these checks are `BORRADO`.

What was not checked: other screens with a repeated confirmation; a dialog that reopens after a timer; the legacy (its session was at the login).

## 0.7.1 — 2026-10-31

**`page find` also searches the option text that a menu field draws.**

What was seen that made it necessary: on the real app, after `fill {Tarifario: "IMPO AIR"}` (answer `ok`), `find {text: "IMPO AIR"}` found nothing: a Vuetify menu keeps a code in its input (`3`) and draws the option text in another element, and `find` looked only at the input.

Now `find` matches the drawn text too and prints it after the code: `f1 [text] Tarifario #input-46 = "3" shows "IMPO AIR"`. Nothing else changes.

How it was checked: the Chrome test of `find` fills a menu field and looks for its option text; it fails without the change and passes with it; 201 tests pass with `-race`. On the real app (ventas) the same `fill` then `find` gave the line above. Also run on the real app with 0.7.0: `open`, `shape` and `find` on sales, purchase and tariff-configuration screens; `parity` of sales and purchase with a Tarifario chosen showed only the real differences (`V-#` and `C-#` fields and buttons ×10, columns `Contrato` and `compra`), while with no Tarifario both screens had 0 columns and no differences; `fill {f1}` by ref; times in `payloads` for XMLHttpRequest calls.

What was not checked: `fill` by ref and parity against the legacy (its session was at the login, and credentials are the user's); `simulate` with a body.

## 0.7.0 — 2026-10-31

**`page find` and element refs, `payloads` filters and times, `code legacy rejects`, `code leftovers ids`.**

What was seen that made it necessary: a colleague testing the tariffs module reported that `state` and `audit` are cut on screens with many checkboxes, that `fill` on legacy inputs with the same name and no id fills the first one, that `payloads` could not be narrowed or read one request at a time and said nothing about time, that the rejection conditions of each `grabar_*.php` were read by hand, and that `ZZ_` rows left by a test could not be cleaned or even named. (Already fixed in 0.6.x and left alone: `batch` output, `code rows` ids and children, `map_back` by route path, `name#2`.)

What changed. `page find {text|regex, max?}` lists only the fields, buttons, headings, alerts, errors and table headers that match, each field and button with a ref (`f3`, `b5`: its place on the screen as it is now) and the dialog or card it is in. `fill {f3: value}` and `click {ref: "b5"}` take a ref; the `fill` warning about several fields with one name now names their refs. `payloads` numbers each request (`#N`), shows the milliseconds it took, and takes `match` (regex over `METHOD address`), `body` (regex over what was sent and got) and `index` (one request, up to 8000 bytes of each body); the line of a request has a new `#N` first and a time last. `code legacy {rejects: true}` adds the throw, die, exit and 4xx/5xx status lines of each PHP file, comments left out. `code leftovers {ids: "id"}` lists the keys of the live test rows (not the soft-deleted ones, 50 at most) and a `DELETE` for the user to run by hand in a demo tenant; webscope still never writes. Every new argument is optional: without it nothing changes except the `#N` and the time in `payloads`.

How it was checked: four new tests that fail without the change (the Chrome test of `find` and refs, the `payloads` filters, `legacy rejects`, `leftovers ids`); 201 tests pass with `-race`, `go vet` and `gofmt`. Checked on the real app (`demo.localhost:5173`): `find` on the roles screen (`b25 [button] Guardar · Módulos`), `payloads {match, limit}` and `{index: 1}` after `open /pricing/sales-tariffs` (`#1 GET … 200 612 ms`), `code legacy {path: comercial, rejects: true}` on 613 files (throw and die lines by file), `code leftovers {ids: "id", deleted}` on `app_tipo_tarifario` (43 rows, 42 marked, live id 554).

What was not checked: `fill` and `click` by ref on the real app (only in the Chrome fixture); refs on a legacy page; `payloads` times for XMLHttpRequest on the real app; a `leftovers` list of more than 50 live ids. Not done from the colleague's list: `simulate` with a defined body, a legacy `open`, selectors by attribute for `fill`, a default tenant for `code rows`, and performance traces or console traces mapped to source (`javascript_tool` gives no access to them).

## 0.6.5 — 2026-10-31

**`page open` waits for the screen to draw its data before it answers.**

What was seen that made it necessary: on the real app a `batch` of `open /configs/roles`, `shape`, `open /configs/roles/1`, `shape` saved the detail screen with `0 fields`, while the same screen read later had 40: `open` answered as soon as the route changed, before the screen had loaded its data, so the next step of a batch read an empty screen. (`page parity {a, b}` of those two shapes worked, in one snippet and one tab, but compared an empty detail.)

Now, after the route changes, `open` waits until the requests the page sent have answered (about 5 s at most) and the text of the page has stayed the same for two looks of 200 ms; it takes about half a second on a screen that is already still. The answer is the same (`opened /path`, or `redirected from …`). A screen that draws without sending a request, after a timer, is not waited for.

How it was checked: the Chrome test of `open` got a screen that draws `ready` when a `fetch` that takes 900 ms ends and a `batch` of `open` and `read`; it failed before (`2. read: ` empty) and passes now, 3 of 3 times; 197 tests pass. Checked on the real app: the same `batch` (`open /configs/roles`, `shape`, `open /configs/roles/1`, `shape`) saved the list with 11 buttons (1 before) and the detail with 40 fields (0 before), and took 1926 ms for the four steps; `page parity {a, b, kinds: "field,column"}` then gave `field switch-# ×40` and the columns `Pantalla` and `Estado` only in the detail. Not checked: a screen that needs more than 5 s of requests.

## 0.6.4 — 2026-10-31

**`page` answers no longer carry the text that `javascript_tool` blocks: an `=` together with a `;` or an `&`.**

What was seen that made it necessary: on the real app the long `batch` of 0.6.2 (`state`, `shape`, `audit` on a screen with 40 checkboxes) came back as `[BLOCKED: Cookie/query string data]`, which 0.5.81 was meant to prevent. Its short answer had two `id=switch-…` and, in the note that the helper itself wrote, a `;`. Strings returned by `javascript_tool` and measured: `id=a1 id=a2`, `x id=a1 y`, `k1=v1 | k2=v2` and `?page=2` pass; `a=1;b=2`, `a=1&b=2`, `id=a1 y; z`, `id=a1 y & z` and `a=1; b` are blocked; a `;` or an `&` with no `=` pass. So the rule of the filter is an `=` and also a `;` or `&` anywhere in the text, not only a chained query string (0.5.81 read it as that).

Now the text that goes to `javascript_tool` has, when it holds an `=` and a `;` or `&`, a comma for each `;` and the word `and` for each `&` (the `=` stays), after the values of chained query strings are hidden as before; the answer says `query values hidden and separators changed` and `page(result)` has the text as it was. The notes that the helper adds (the pointer to `page(result)`, the summary of a `batch`, the lines about steps not run) have no `;` and no `&`. The changed text is stored in full like any other long answer.

How it was checked: the Chrome test of the result channel got a paragraph `campo id=a1 y; z & w`; it failed before (the answer came back as it was) and passes (`id=a1 y, z and w`), and the test now fails if any answer holds an `=` with a `;` or `&`; it ran 3 of 3 times; 197 tests pass. Checked on the real app: the batch of `state`, `shape` and `audit` on the roles screen (40 checkboxes), blocked before, came back with the three lines and the pointer, and `page(result)` with `offset: 1500` gave the end of `state` and the whole `shape` and `audit`. Known limit: the filter was measured with 12 strings, not read, so another pattern may exist.

## 0.6.3 — 2026-10-31

**`code map_back` finds a module by the route path of one of its screens (`sales-tariffs/bulk-upload`).**

What was seen that made it necessary: `map_back {query: "sales-tariffs/bulk-upload"}` answered `no module or route contains «sales-tariffs/bulk-upload»` (seen on the real project). That text is the path of a screen of the front; `map_back` looked for it in the names of the front modules (`crm/tariffs`) and in the addresses of the backend (`tariffs/{}/{}/bulk/import`), and the path of a screen is in neither.

Now a query with a slash that is part of the path of a route of a front module (compared without case and without the slashes at its ends, so `sales-tariffs/bulk-upload`, `Sales-Tariffs/Bulk-Upload` and `/pricing/sales-tariffs/bulk-upload/:tariffTypeId` all work) finds that module, and the answer is the one that the name of the module would have given: all of its routes, then the endpoints its calls reach. A query without a slash is read as before, so `tariff` finds the same modules. `code(help)` says it.

How it was checked: a new test (a module `crm/tariffs` whose route path has no word of its name in common with the query) failed with the message above for the three forms and passes; a query for another module does not bring it, and a path that no route has is still not found. 197 tests pass. Checked on the real project: `map_back {query: "sales-tariffs/bulk-upload"}` answered the module `crm/tariffs` with its 6 routes, 3 endpoints (`create-quote`, `tariff-columns`, `bulk/interpret`), 4 services and 12 models.

## 0.6.2 — 2026-10-31

**A long `batch` shows the first line of each of its steps to `javascript_tool`, not only the start of the first one.**

What was seen that made it necessary: a `batch` of `state`, `shape` and `audit` on a screen with 40 checkboxes came back with the first 23 checkboxes of `state` and `(… 1500 more chars …)`: the `shape` answer (`saved: 40 fields…`) and the `audit` were not visible until `page(result)` was called, and a user read that as the later steps being lost (they were run; the whole text was in `page(result)`).

Now, when the answer of a `batch` is longer than what `javascript_tool` shows, the short answer is the first line of each step (`1. state: URL: …`, `2. shape: shape «zz-roles» saved: 40 fields, …`, `3. audit: no label (40): …`, each cut at 110 characters) followed by `(first line of each step; javascript_tool cuts at 1000, page(result) gives all)`. If those lines do not fit either, it falls back to the start of the answer, as in 0.5.81. The stored answer is the whole one, as before.

How it was checked: the Chrome test of the result channel got a batch of a long `read`, a short `read` and a `shape`; the short answer failed before (it showed only the long read) and now has the three lines and the pointer, under 1000 characters; it ran 2 of 2 times; 196 tests pass. On the real app the batch of `state`, `shape` and `audit` before this change showed only `1. state` (seen); the new answer, first run, came back `[BLOCKED: Cookie/query string data]` (see 0.6.4); after 0.6.4 it shows `1. state: URL: …`, `2. shape: shape «zz-roles» saved: 40 fields, …`, `3. audit: no label (40): …` and the pointer, and `page(result)` has the whole text.

## 0.6.1 — 2026-10-31

**A missing table in a call with no `tenant` now says to pass the tenant and names the host names that the central database knows, in `query`, `code` and `check`.**

What was seen that made it necessary: `code rows {table: app_tarifario_prov, a: 1, b: 2}` failed with `Table 'acme.app_tarifario_prov' doesn't exist` (the tables of the app are in the tenant's database, and the call read the central one). On the real server `code` already said `pass tenant to read the one of the app`, but `query` and `check` said only `Table … doesn't exist`, and none said which value to pass: the host name (`demo.localhost`, not `demo` and not the uuid) had to be found in `hostnames` by hand.

Now, when the error is a missing table (`Error 1146` or `no such table`), the call named no tenant and the server uses the default tenant lookup of Hyn Tenancy, the error ends with `; the call named no tenant, so it read the central database: pass tenant to read the one of the app (host names: demo.localhost)` (up to 4 names, read with `SELECT fqdn FROM hostnames WHERE deleted_at IS NULL`). With a tenant, with another error or with a `WEBSCOPE_TENANT_QUERY` of its own (the layout is not known) the hint does not come or has no names. The tenant is not guessed: the call still fails and the model decides.

How it was checked: a new test in `db` (no tenant, a table that is not in the central database: the hint and the first host names; with a tenant, a missing column and a tenant query of its own: none) failed before and passes; a test in `check` for the wiring; 196 tests pass. On the real server: `code rows {app_tarifario_prov}` and `query SELECT id FROM app_tarifario_prov` with no tenant ended with `pass tenant to read the one of the app (host names: demo.localhost)`, and `rows` with `tenant: demo.localhost` compared the rows 1 and 2. Not checked: `check` on the real server; a central database with several tenants.

## 0.6.0 — 2026-10-31

**0.6.0 gathers 0.5.65 to 0.5.86: the model no longer types addresses by hand, long answers do not get lost, and the data tools tell what to ask next.** The four tools stay (`query`, `page`, `check`, `code`) and `tools/list` does not grow (1541 bytes at 0.5.65, 1540 at 0.6.0, measured; the budget is 1800); every change below is an optional action or argument, so a call that worked before gives the same answer, except where an entry says that what a tool prints changes.

What was seen that made it a minor version: two users ran the e2e of a module with 0.5.64 and reported what cost them calls, time and wrong results. Each friction was measured before it was changed, on the real app, the real database or a real Chrome, and each change has a test that failed before it. The result:

- **Screens (`page`).** `open {path}` goes to a router path through the app's router and refuses a path that no route owns (the tenant prefix was why a typed address ended in the not-found screen). `options {field, match?, max?, fresh?}` lists the choices of a select or menu: it reads a long list from its top without skipping a row, remembers a list read to its end for 2 minutes, and says `may have more` when it did not see the end (856 cities: the first read 8 s, the next ones 1 ms). `batch {steps}` runs several snippet actions in one `javascript_tool` call and stops before the 45 s limit. `result` gives the whole answer that the helper kept when `javascript_tool` would have cut it at 1000 characters (`[TRUNCATED]`) or blocked it for chained query values (`[BLOCKED: Cookie/query string data]`). `fill` takes `name#2` for the second field with a name. `payloads` says how old its last request is and when to load the helper again. `parity {req_a, req_b, map}` says how many names of the map matched. `download wait` reads the headers of a text sheet (HTML table, tabs, semicolons) as it does of an xlsx.
- **Data (`check`, `code`, `query`).** `check` accepts numbers in `where` and, with no `map`, lists the sent fields, the columns and the names they share. `code map_back` pages its endpoints with `offset`, keeps room for endpoints on the first page and drops services and models in turn. `code rows` takes ids (`key`), names the columns whose values differ in child rows and lists the real columns when one is wrong. `code leftovers` counts several columns and, with `deleted {estado: BORRADO}`, tells the soft-deleted rows from the live ones.
- **What changes in what a tool prints.** `payloads` has `; last one N s ago` on its first line; the cut line of `map_back` says `offset N`; `leftovers` with `deleted` has the counts of marked and live rows; a long answer of a `page` snippet ends in a pointer to `page(result)` and chained query values are hidden in it; `check` with no `map` answers with the names instead of an error.
- **What 0.6.0 does not do.** A first click on a confirmation that did nothing (reported once) was not reproduced and is not changed. The legacy pages that load their lists only through `MDI_CCIAL.php` were not tried. A list of more than 400 scroll steps is cut and says so. `check` was not run end to end against a real row (it needs a real write). The text-sheet headers of `download` were not run on a real legacy file. Clean-up of `ZZ_` rows is still the job of the person.

How it was checked: 194 tests with `-race` pass, `go vet` and `gofmt` are clean, and each entry from 0.5.65 on says what was run on the real app and what was not. Checked on the real app for this release: `state` of a screen with 40 checkboxes through `page(result)`, chained query values, `Proveedor#2`, `parity` with a map, `rows` with child rows of equal count, `leftovers` with `deleted`, `batch`, `options` and `check` without a map.

## 0.5.86 — 2026-10-31

**`page download` gives the headers and the row count of a text sheet (an HTML table or lines with tabs, semicolons or commas), also in Latin-1.**

What was seen that made it necessary: a user's legacy `.xls` was answered with only `type text` and no headers, so an export could not be checked against the new one. A first test showed that the same file written in Latin-1 (what a legacy export writes) was not even called text: `type unknown`.

Now a file with no NUL byte and under 2% control characters is text (UTF-8 or Latin-1). For it, `download wait` reads up to 512 KB and, if it has a `<table`, gives the cells of the first row as `headers:` and the number of rows (`<tr`); if not, it takes the first line and splits it by tab, else by `;`, else by `,`. The answer adds `headers: Cotización, Cliente` and `rows: 3 (header included)`; each header is cut at 40 characters, 30 at most; no value is printed. A text with no table and no separator (one line of prose) gets no headers.

How it was checked: a new test with an HTML `.xls` in Latin-1, tab text, `;` text and plain text failed before (no headers; `type unknown`) and passes: the headers and the count come back, a cell value never shows, and the plain text is given no headers. The download tests pass (13 s); 194 tests pass. Not checked on the real legacy `.xls`. Known limit: a text of prose with commas in its first line is read as a CSV and its words as headers.

## 0.5.85 — 2026-10-31

**`code rows` takes ids for `a` and `b`, names the columns of the child rows that differ, and lists the real columns when a child column is wrong.**

What was seen that made it necessary: `rows` refused `a: 1, b: 2` (it needed `{id: 1}`), `children` only counted the rows that point at each (`a 0, b 5`), and a child column that did not exist answered with the database's `Unknown column 'id_tarifario'`, with no list of the real ones.

Now `a` and `b` may be an id (text or number); the column is `key`, `id` by default (`rows {table, a: 1, b: 2}`, or `key: "name"`). The `{column: value}` maps work as before. For each table of `children`, when both sides have the same number of rows (at most 200), the line adds `; values differ in: qty` (the columns whose sorted values differ, the first column of the child table, its id, left out) or `; the same values in every column`; with different counts it is `a 2, b 1 ≠` as before. A child column that does not exist is refused with `"no_such" is not a column; columns of kid: id, par_id, qty, label`. No value is printed.

How it was checked: a new test (two parents with two children each, one column of values different, one equal; ids as text and as a number; `key`; different counts; a wrong child column; an invalid key; the maps) failed before (`rows needs a: {column: value} that picks one row`) and passes; the first version compared the id column of the child rows too and was fixed before this commit. 192 tests pass. Checked on the real database: `rows` of `app_tarifario` 1 and 2 with `children {app_tarifario_servicios: id_row_tarifario}` gave `a 0, b 5 ≠` with the old code; on the real database the new code gave the same `a 0, b 5 ≠` for 1 and 2 (`rows` with `a: 1, b: 2` worked), `a 5, b 5; values differ in: valor` for the tariffs 2 and 1346, and for the column `id_tarifario` that does not exist `"id_tarifario" is not a column; columns of app_tarifario_servicios: id, id_row_tarifario, id_servicio, id_variacion, valor`.

## 0.5.84 — 2026-10-31

**`page parity {req_a, req_b, map}` says how many entries of the map found their name, and names the ones that did not.**

What was seen that made it necessary: a `map` renames the names of a before they are compared, and an entry with a wrong name changed nothing and showed nothing: the wrong name just appeared as «only in a» with no hint that the rename had not applied.

Now, when there is a `map`, the answer has `map: X of N names matched`, and one line for each entry that did not: `«name»: not in a` (a has no such name, in what it sent or got) or `«name» → «other»: not in b` (a has it, b has no name `other`). Names only, never a value. Without a `map` there is no such line.

How it was checked: a new test (3 entries, 1 matched; 2 of 2; no map) failed before (no `map:` line; `valor_mal` showed only as «sent only in a») and passes; the earlier parity tests pass. 192 tests pass. Checked on the real app with two requests that the dev server answered 404 (field names only): `map: 1 of 3 names matched`, `«nada»: not in a`, `«valor» → «valor_mal»: not in b`.

## 0.5.83 — 2026-10-31

**`code leftovers` takes `deleted {column: value}` and tells the rows that carry the mark of a soft delete from the live ones.**

What was seen that made it necessary: `leftovers` counted every row whose column starts with `ZZ_`, also the ones the screen had already deleted (the app marks them `estado = BORRADO` and keeps them), so a clean test still ended with «they are test rows: clean them through the screen».

Now `deleted: {estado: "BORRADO"}` adds to each count how many of those rows carry that mark: `… : 3 rows (2 marked estado=BORRADO, 1 live)`. The warning to clean through the screen comes only when some are live; when all are marked it says `Every one of them is marked estado=BORRADO: nothing is left to clean.` Without `deleted` the answer is the one it always was. One entry only; the column a plain name, the value 1 to 40 letters, digits, `_` or `-`; anything else is refused.

How it was checked: a new test (3 rows with 2 marked; 2 rows all marked; no `deleted`; four refused forms) failed before and passes; 190 tests pass. On the real database no `ZZ_` row of `tm_proveedores` is `BORRADO` (the one there is `ACTIVO`), so the real case was not run; only that the real counts have not changed is known. Checked on the real database that the answer has the new shape: `tm_proveedores.nombre` gave `1 rows (0 marked estado=BORRADO, 1 live)`; not checked with a row that is marked.

## 0.5.82 — 2026-10-31

**`fill` and `options` take `name#2`, `name#3` to pick one of several fields with the same name.**

What was seen that made it necessary: `fill` warned that two fields answered to `Subtipo`, `Servicio` or `Estado` and the only way to reach the second one was to find its id (the legacy inputs of a service have none).

Now a name that answers to nothing as written is read as `name#N`: the Nth field that answers to `name`, in the order `fill` already uses (a dialog's fields first, then the page's). A name that answers to something is not changed, so a label that really ends in `#2` still works; the warning of a name that fits several fields now says `or write the name with #2 for the second`. `options` takes the same names.

How it was checked: a Chrome test on the fixture, which has a «Nota» in a dialog and another in the page: `Nota` warns and fills the dialog's; `Nota#2` fills the page's (the page's field tells «outside») with no warning; `Nota#3` is not found. It failed before (`Nota#2: not found`) and the page tests pass (28 s). Checked on the real app (new tariff form, two fields called «Proveedor», the page's filter and the form's): `Proveedor` warned `2 fields answer to this name; filled #input-165, the others are #input-49` and the warning now mentions `#2`; `Proveedor#2` picked the other one (`#input-49`; its menu would not open behind the modal form, so the fill failed with `no option … among 0 seen`, which is not a fault of the selection); `Proveedor#3` answered `not found`. The inputs of the legacy service without id are not checked at all (they need the legacy page).

## 0.5.81 — 2026-10-31

**New `page result`: the helper keeps the whole answer of a snippet when `javascript_tool` would cut it or block it, and returns a short one.**

What was seen that made it necessary: two users reported `state` ending in `[TRUNCATED]` and answers replaced by `[BLOCKED: Cookie/query string data]` (3 and 4 times). Measured on the real app with fixed strings returned by `javascript_tool`: it cuts what it returns at exactly 1000 characters and puts `[TRUNCATED]` (the default cap of `state` and `read` is 3000, so any long screen was cut, with 40 checkboxes or none); it blocks text with chained query values (`a=1&b=2`, `?page=2&token=abc`) but not a single `?page=2`, nor `?page&token`, nor e-mail addresses or plain URLs. (That was read too narrowly: 0.6.4 measured that the filter blocks any text with an `=` and also a `;` or an `&`.)

Now the helper loads with the limit (`k`, 900 characters by default; 0 turns it off). When the answer of an action is longer, or has chained query values, the helper posts the whole answer to the local listener (the same token and origin check as the captures) and returns a short one: the first lines, cut at a line, with `(… N more chars: javascript_tool cuts at 1000, page(result) gives all)`; the values of chained query strings are hidden in what it returns (`a&b&c`; the stored answer keeps them) and it says so. `page(result)` gives the stored answer, in parts of about 8000 bytes with `result {offset}` for the next. If the listener cannot be reached, the answer is returned as it was. The steps of a `batch` answer to the batch, which is the answer that is kept. Only the last answer is kept.

How it was checked: a Go test for the route and the action (no token and another origin refused, an answer in parts that add up to the whole, a new one replaces the old) and a Chrome test (a short answer unchanged; a 3200-character one cut under 1000 with the pointer; `?a=1&b=2&c=3` returned as `?a&b&c` with the note and kept as it is). 189 tests pass. Checked on the real app (roles screen with 40 checkboxes): `state` came back to `javascript_tool` as 23 checkboxes and `(… 933 more chars: javascript_tool cuts at 1000, page(result) gives all)`, with no `[TRUNCATED]`, and `page(result)` gave the 40 checkboxes, the buttons and `Tables (rows): 40`; a paragraph with `https://empresa.com/ver?page=2&token=abc&q=x` was returned as `?page&token&q` with `query values hidden; page(result) gives the answer as it is` (the same kind of text was `[BLOCKED]` before), and `page(result)` had it with the values. Not checked: a second answer replacing the first on the real app; the helper loaded with the limit off.

## 0.5.80 — 2026-10-31

**`code map_back` drops services and models in turn when they do not fit, so both stay.**

What was seen that made it necessary: in 0.5.73 the first page of `map_back tariff` gave room to the endpoints by dropping the services first: all 17 services went before a single model (`29 more services and models not shown`), and the answer had no service line at all.

Now lines are dropped from the end, one at a time, always from the kind (service or model) that has more lines left, so the two stay in proportion. The closing line is the same (`(… N more services and models not shown: narrow the query)`).

How it was checked: the test of 0.5.73 (80 services, 80 models, 100 endpoints) got a new assertion that failed (`0 services, 58 models`) and passes now (services are at least a third of the models). 187 tests pass. Checked on the real project: `map_back tariff` showed 13 endpoints, all 17 services and 22 models, and `27 more services and models not shown` (0.5.73 showed 0 services and 29 models dropped).

## 0.5.79 — 2026-10-31

**`check` without `map` lists the fields that were sent and the columns of the table, instead of failing.**

What was seen that made it necessary: to use `check` the names of the sent fields and of the columns had to be known first, and a call without `map` answered `where and map need at least one entry each`, which cost a call to `payloads` and another to `code fields` before the real one.

Now a call with `sent` and `table` but no `map` answers `no map given. Sent fields: …. Columns of <table>: …. Same name on both sides: …. Next: check {sent, table, where, map {sent field: column}}; where {column: value} picks the row.` (the names of the top-level fields of the body that was captured, the columns read with a read-only `SELECT … LIMIT 0`, and the names they share; no value). A call with a `map` and no `where` still fails as before; so do a missing `sent`, an invalid table name and a body that is not JSON or was cut.

How it was checked: a new test fails before (`where and map need at least one entry each`) and passes: with and without `where`, the three lists, no value shown, and the refusals that stay. The test that expected the error for «no map» now expects the list. 187 tests pass. Checked on the real MySQL (tenant `demo.localhost`, table `app_tarifario`): with a captured `POST` of four fields it answered the four sent names, the 11 columns and the two that are the same (`ciudad_origen`, `estado`), with no value. The request was one answered by `simulate` with a 503, which the listing does not mind (the comparison does).

## 0.5.78 — 2026-10-31

**New `page batch {steps, keepGoing?}`: several snippet actions in one snippet, so one `javascript_tool` call replaces two calls per action.**

What was seen that made it necessary: every `page` action costs two calls (the server gives a snippet, `javascript_tool` runs it), and a run of `javascript_tool` is cut at about 45 s (it expired when six `options` calls were chained by hand).

Now `batch` takes `steps: [{action, args}, …]` (the actions that return a snippet: `state`, `read`, `fill`, `click`, `press`, `simulate`, `open`, `options`, `audit`, `shape`) and returns ONE snippet that runs them in order. The answer lists them numbered (`1. fill: Nombre: ok`, `2. read: Ana`). A step that fails (an answer that starts with `Failed:`) stops the rest and says how many were not run, unless `keepGoing: true`. No step starts after 38 s; the answer then says how many are left to run in another call. `payloads`, `parity`, `download` and `verify` are answered by the server and are refused as steps, with the ones that are allowed. A batch inside a batch is refused. `page help` grows by about 450 bytes; `tools/list` does not change.

How it was checked: a Go test for the refusals and for the arguments travelling as JSON (a `</script>` in a value), and a Chrome test (fill and read in one batch; a failed click that stops the next step; the same with `keepGoing`); both failed before and pass now; 186 tests pass. Checked on the real app: one `javascript_tool` run did `fill` of `Tarifario`, `click` of `Nueva Tarifa`, `options` of `Proveedor` (3 suppliers) and an `options` of a field that does not exist, whose failure stopped the last step (`state`) with `(1 steps not run after the failure; keepGoing:true runs them anyway)`. Not checked: the 38 s limit against a real long step; `keepGoing` on the real app.

## 0.5.77 — 2026-10-31

**`page options` remembers a menu that was read to its end for 2 minutes, and scrolls faster.**

What was seen that made it necessary: on the real form every call to `options` took about 16 s on a list of 856 rows, even when the question was a second one on the same field (`AAC`, then `ZHENG`), because the field does not filter its list and each call scrolled the whole list again.

Now a list that was seen to its end is kept for 2 minutes, by field, and the next question on that field (any `match`, any `max`) is answered from it without opening the menu; the answer ends with `(read N s ago: fresh:true reads it again)`, and `fresh: true` reads the menu again. Only an unfiltered list is kept (a search typed into an autocomplete filters its list: that one is not). The scroll loop no longer waits a fixed 70 ms once it is scrolling (it already waits for the page to draw) and makes its step longer, up to 90% of the rows drawn, when half of the rows read were already known; a step with no row in common still goes back and shrinks.

How it was checked: in the Chrome test, a second question on the 2000-row list gives the 111 rows of `R19`, says how old the list is and leaves the menu opened once (a counter of presses on the field); with `fresh: true` it opens it a second time and gives the same rows. The earlier steps of that test that need the menu use `fresh: true`. The test ran 3 of 3 times; 184 tests pass. Checked on the real form (856 cities): the first read, `match "AAC"`, took 8.0 s (15.7 s in 0.5.76) and answered `AACHEN`; then `match "ZHENG"` and `max 2000` answered in 1 ms from memory (`ZHENGJIANG | ZHENGZHOU`; 856 rows, `read 5 s ago`); `fresh: true` read the 856 rows again in 4.4 s. Not checked: a list longer than 400 steps; a list that changes inside the 2 minutes without `fresh`.

## 0.5.76 — 2026-10-31

**`page options` waits for the page to draw the rows after each scroll and goes back when a step left a gap, so it does not skip rows.**

What was seen that made it necessary: on the real form the list of cities has 856 rows (read with slow steps of 40% of the box and 140 ms: 856). With the step of 0.5.75 (60% of the rows drawn, a fixed 70 ms) `options` read 846 and with a step of 90% it read 856, so the same call could miss rows depending on how late the page drew them. The same risk was in 0.5.72 to 0.5.75 (an 80% step with a fixed wait). The time per call was 9 to 13 s on that list.

Now, after each scroll, it waits (up to about 480 ms) until the first row drawn changes, reads the rows and, if none of them was in the window before the step, goes back half the step and halves the step size (down to 10% of the rows drawn). The limit of 400 steps stays.

How it was checked: the Chrome test list of 2000 rows draws its rows only after the list has been still for 300 ms; the question `R19` (111 rows) must give all 111. With the loop of 0.5.75 it failed in some runs (59, 101, 103 of 111; the reproduction depends on the timing of Chrome) and passed in others; with this loop it passed in 6 of 6 runs. The 184 tests pass. Checked on the real form (856 cities): `options` with `max 2000` and no `match` answered exactly 856 rows, from `6TH OF OCTOBER` to `ZZ_E2E_TRY_CIUDAD`, with no «may have more», in 22 s the first time and 15.8 s the second (856 both times). `match "AAC"` answered `AACHEN` in 15.7 s and `match "ZHENG"` answered `ZHENGJIANG | ZHENGZHOU` in 15.8 s. That is slower than 0.5.75 (9 to 13 s), and is the price of reading the whole list without gaps: every call with `match` scrolls the whole list, because this field does not filter its list when a search is typed. A list of 5000 rows would need about 90 s and reach the limit of 400 steps; use a narrower question there. Not checked: a list that long.

## 0.5.75 — 2026-10-31

**`page options` scrolls a menu by the rows already drawn, so a long list is read in far fewer steps.**

What was seen that made it necessary: on the real form (582 cities) each `options` with `match` took about 12 s for one or two rows, and a list of more than about 330 rows was cut at the limit of 400 steps (`PRAGA`). Each step moved the list by 80% of its box (248 px, 3 or 4 rows) while Vuetify keeps about 62 rows (4000 px) drawn around the box.

Now a step is the larger of 80% of the box and 60% of the span of the rows drawn, so no row is left out between steps. The limit of 400 steps stays; it now covers a much longer list. `fill` is not changed.

How it was checked: the Chrome test got a list of 2000 rows (40 drawn, a box of 120 px) and asks for `R1999`; it failed (`(none) … may have more`, cut at 400 steps) and passes now. The 184 tests pass. Checked on the real form: it did not make the calls faster (9.3 to 13.5 s for `AAC` and `ZHENG`), and it could skip rows (846 rows read against 856 in the list); both are in 0.5.76.

## 0.5.74 — 2026-10-31

**`page options` closes a menu that is open before it opens it, and leaves it closed.**

What was seen that made it necessary: on the real form, `options {field: "Ciudad de origen", match: "AAC"}` answered `AACHEN` the first time and `(none)` when the same call was repeated, and also after a call with another `match` on that field. Looking at the page between calls showed the cause: the menu of the first call was still open (a list with 62 rows) and pressing a field that has its menu open closes it, so the second call typed its search with no menu.

Now `options` closes an open menu first (Escape on the field and a click outside, waiting up to about a second for its rows to go) and does the same when it ends, so the next call finds it closed. For an autocomplete or combobox with `match` it also sets the search to an empty text, waits 300 ms and then types `match`. `fill` is not changed.

How it was checked: the Chrome test got an autocomplete that toggles its menu on a press of the field, lists a search only while the menu is open and stays open on a click outside; the second identical call failed with `Ciudad: (none)` (as on the real form) and both pass now. The 184 tests pass. A first version of this change (only the empty search before typing) did not fix the real form: the same `AAC` call still gave `(none)` the second time; that commit was replaced before being pushed. Checked on the real form: `match "AAC"`, `"AAC"`, `"ZHENG"`, `"AAC"` in four separate calls answered `AACHEN`, `AACHEN`, `ZHENGJIANG | ZHENGZHOU`, `AACHEN`. Each took about 12 s: the field did not filter its list when the search was typed (the list kept its 62 rows and 38491 px), so `match` was found by scrolling the whole list; see 0.5.75.

## 0.5.73 — 2026-10-31

**`code map_back` keeps at least 3000 bytes for endpoints on the first page; the services and models give way.**

What was seen that made it necessary: on the real project, `map_back tariff` showed 3 endpoints and `34 more`, because the front routes, the services and 53 models (many of them models of other modules that the services import) took almost all of the 8000 bytes.

Now, on the first page (`offset` 0), if the services and models would leave the endpoints less than 3000 bytes (or less than what they need, when they are fewer), lines of services are dropped from the end first, then lines of models, and the last line says `(… N more services and models not shown: narrow the query)`. Pages after the first are as in 0.5.68. A project whose services and models fit is answered as before.

How it was checked: a new test (a controller that reaches 80 services and 80 models, and 100 more endpoints) failed (1 endpoint on the first page, the answer cut in the middle of the models) and passes now (20 or more endpoints, some tables kept, the cut said, at most 8000 bytes plus the closing line); the earlier test that the tables survive 400 endpoints passes too. The 184 tests pass. Checked on the real project: `map_back tariff` showed 13 endpoints on the first page (3 before) and `24 more … offset 13`; `offset 13` showed 24 endpoints with none of the first 13 (13+24 = 37, as before); the first page dropped all 17 services and 29 models (`29 more services and models not shown`). The dropped models are not reachable by `offset`: only a narrower query shows them; all the services went before any model.

## 0.5.72 — 2026-10-31

**`page options` starts at the top of a menu and says «may have more» when it did not see the whole list.**

What was seen that made it necessary: on the real form (582 cities) `options` with `max 5` and no `match` answered `ZHENGZHOU … ZHUHAI` (the menu opened scrolled down) and, with `max 1` on a short list, the count of `more` was only the rows seen so far, so on a long list it would understate the total.

Now `options` scrolls the newest menu to its top before it reads, and the `(… N more …)` count is given only when the list was seen to its end (a native select, or a menu scrolled to the bottom). When it stopped because it had `max` rows and had not seen the end, the answer says `(… may have more: narrow with match, or raise max)`. `fill` is not changed.

How it was checked: the Chrome test of `options` got a menu that opens scrolled to row 120 and a native select with `max 1`; against the old `page.js` it failed with `PAIS 120 … (… 1 more …)` and with the new one it passes (`PAIS 000 … 004`, `may have more`; `Tipo: uno (… 1 more …)`). The 183 tests pass. Checked on the real screen: `fill` of `Proveedor` with a tariff name right after `fill` and after `options` of `Tarifario` answered `no option … among 3 seen` both times: the rows of the other menu did not mix into `fill` there, so it was left as it is. Checked on the real form (582 cities): `max 5` with no `match` gave `6TH OF OCTOBER | AACHEN | AARHUS | ABIYAN | ABU DABI (… may have more …)`, from the top. With `max 600` the list stopped at `PRAGA` after 400 steps (about 330 of the 582 rows) and said `may have more`: a list that long needs `match`. Seen on the same form: the same `match` twice gave `(none)` the second time (0.5.74).

## 0.5.71 — 2026-10-31

**`page options` lists only the rows of the newest menu.**

What was seen that made it necessary: on the real tariffs screen, `options` of `Proveedor` right after `options` of `Tarifario` answered the 8 tariff names followed by the 3 suppliers: the menu of the first field was still in the page, and `options` read every row of the overlay container.

Now it reads the rows of the last `.v-list` only, which is the one just opened. `fill` is not changed (it reads all the rows; a menu that is leaving could mislead it in the same way, and that was not checked).

How it was checked: the Chrome test of `options` got a step with an older menu earlier in the page (`STALE row`); it failed (`Origen: STALE row`) and passes now. On the real tariffs screen the old helper mixed the rows; with this version `Tarifario`, then `Proveedor`, then each again with `match "ZZ"` gave only the rows of each field (8 tariffs, 3 suppliers, `ZZ_E2E_TARIFF`, `ZZ_E2E Proveedor 2`). The 183 tests pass. Not checked: `fill` after another menu.

## 0.5.70 — 2026-10-31

**`code leftovers` takes `columns` (up to 10) and counts each one on its own line.**

What was seen that made it necessary: `leftovers` took one table and one column per call, so finding which column of a table held the `ZZ_` rows took one call per guess.

Now `columns: ["code", "name", …]` (alone or with `column`) answers one `leftovers table.col starting with ZZ_: N rows` line per column, and the warning to clean through the screen comes once, at the end, when some column has rows. With one `column` the answer is the same as before. More than 10 columns, or an invalid name, is refused. Only counts: no value is shown.

How it was checked: a new test failed (`leftovers needs table and column`) and passes now (3 columns: 2, 2 and 0 rows, the warning once, no value, a bad name and 11 columns refused); the earlier leftovers test still passes. The 183 tests pass. Checked on the real MySQL (tenant `demo.localhost`): 4 columns of `tm_proveedores` in one call gave 1 row in `nombre` and 0 in the other 3, the warning once; one `column` gave the same line as before.

## 0.5.69 — 2026-10-31

**New `page options {field, match?, max?}`: the choices of a select or menu, without choosing one.**

What was seen that made it necessary: when `fill` found no option it listed the first 5 and the count (`no option "AACHEN" among 582 seen`), and there was no way to see the rest or to look for a name.

Now `options` gives all the options of a native select, and of a Vuetify menu the rows seen while it scrolls (an autocomplete or combobox is searched with `match`). With `match` only the rows that contain it are kept; `max` (30 by default) stops the list and says how many more there are. A field that does not exist fails naming it; a field that is not a select or menu says so. `fill` and its error are unchanged. `page help` grows by about 300 bytes; `tools/list` is unchanged.

How it was checked: a new Chrome test fails without the action and passes with it, on a native select, a virtual list of 300 rows (with `match` and with `max`) and an autocomplete that answers a search. The 182 tests pass. The first real use (0.5.69) mixed the rows of two menus; see 0.5.71. Checked on the real form (new tariff, 582 cities, nothing saved: only 2 GET requests were captured): `Ciudad de origen` with `match "AAC"` gave `AACHEN`, with `max 5` it gave 5 rows and `5 more`, `País de origen` with `match "ALEM"` gave `ALEMANIA`. Seen: without `match`, a long list gives the rows in the order they were seen, not the first ones alphabetically (the 5 rows were `ZHENGZHOU` … `ZHUHAI`, the menu opened scrolled; the cause was not found). Not checked: a list longer than 400 scroll steps.

## 0.5.68 — 2026-10-31

**`code map_back` takes `offset` to page through endpoints.**

What was seen that made it necessary: when the endpoints did not fit in 8000 bytes, `map_back` said `… and 34 more endpoints not shown: narrow the query` and there was no way to see them except guessing a narrower query.

Now the cut line reads `(… and N more endpoints not shown: offset M for the next page, or narrow the query)`, and `offset M` starts the list at endpoint M. A page after the first leaves out the services and models (the first page gave them), so it fits more endpoints. An `offset` past the end shows no endpoints and no cut line. `code(help)` says it.

How it was checked: a new test (406 endpoints) failed (one page showed 59) and passes now: following the offsets shows every endpoint exactly once, and later pages carry no `model` line. The 181 tests pass. Checked on the real project: `map_back tariff` showed 3 endpoints and `34 more … offset 3`; `offset 3` showed the other 34, none of the first 3, and no services or models. Not checked: that the endpoints of the second page equal the ones of `flows` one by one. Seen, not changed: the first page shows only 3 endpoints because the services and models take most of the room.

## 0.5.67 — 2026-10-31

**`check` accepts a number as the value of a `where` entry.**

What was seen that made it necessary: `check` with `where {"id": 1341}` failed with the Go text `json: cannot unmarshal number into Go struct field checkArgs.where.id of type string`; only `{"id": "1341"}` worked.

Now a JSON number is read as the digits it was written with (`1341` is `"1341"`, a long number keeps all its digits, a leading zero still needs a text). `null`, `true`, `false`, an object or a list fail with `where "id" must be a text or a number`; the message is Go's, not the `Failed: … Next: …` form, because the arguments are refused before `check` runs.

How it was checked: a new test failed with the exact Go text above and passes now; the 14 tests of the package pass. Checked on the real server: `check` with `where {"id": 1}` and `tenant demo.localhost` no longer fails with the Go text; it reached the next step (`the sent body is not JSON`, the captured request was a GET). Not checked: the comparison itself against a real row (no JSON body was captured without sending a write); the tenant of `check` and `query` is the `fqdn` of the host (`demo.localhost`), not `demo` nor the uuid.

## 0.5.66 — 2026-10-31

**`page payloads` says how long ago the last request was captured, and after 30 s adds how to bring the helper back.**

What was seen that made it necessary: after a full reload of the tab without `page(load)`, `payloads` kept saying «23 requests captured» and showed the last request of the screen before, with nothing that told the helper was gone. A user compared the endpoints of two screens with it.

Now the first line ends with `; last one N s ago` (only when something was captured). From 30 s on it adds `If the page was reloaded since, the helper is gone and nothing new is captured: run page(load) again`. A quiet screen gets the same hint: the server cannot tell a quiet page from a reloaded one, so the text says «if».

How it was checked: a new test with a fake clock fails without the change (it does not build) and passes with it (5 s: age and no hint; 95 s: age and hint); vet and the suite pass. Checked on the real screen (tenant `demo`): with the helper loaded `payloads` said `last one 2 s ago`; after a full reload with no `page(load)` the helper did not exist (`window.__ws` was undefined) and, 45 s later, `payloads` said `last one 45 s ago. If the page was reloaded since… run page(load) again`. A quiet page with the helper (47 s) gets the same hint: the text says «if» for that reason.

## 0.5.65 — 2026-10-31

**New `page open {path}`: goes to a router path through the app's own Vue Router and refuses a path that no route owns.**

What was seen that made it necessary: the model typed the address of a screen taken from `code map` (`/pricing/sales-tariffs`). In the real app that address is a router path, and the URL needs the tenant as its first segment (`router.ts`: `createWebHistory('/<tenant>/')`). The browser took `pricing` as the tenant and the app showed its not-found screen, then the login. Read from the live router: `resolve('/pricing/sales-tariffs').href` was `/pricing/pricing/sales-tariffs` under that wrong base.

Now `open` resolves the path with the router of the page. A known path is opened (the app adds its base) and the answer says where it ended, with `redirected from … : session or permission?` when it is not the path asked. A path that only the catch-all route matches fails with up to 8 similar paths. Without a Vue Router in the page it says so. `help` lists it (about 300 bytes more); `tools/list` is unchanged.

Checked: a new Chrome test (a fake router) fails without the action and passes with it; vet and the 178 tests pass. Checked on the real screen (tenant `demo`, session started by the user): `open /pricing/sales-tariffs` and `/pricing/purchase-tariffs` ended at `/demo/pricing/…`; `/pricing/sales-tariffs/999999` was opened (the router owns the pattern, the record is not looked up); `/pricing/sales-tariff` and `/crm/tariffs` (the folder of the module, not a route) were refused with the similar paths; before login, `/pricing/sales-tariffs` was refused (no route without a tenant). Sizes seen on that screen: `state` 1096 bytes, `read` 382, `audit` 166. Not checked: the `redirected from` text in the real app, and a router that is not Vue Router.

## 0.5.64 — 2026-10-31

**An `opaqueredirect` answer (a `redirect: manual` request) is also shown as opaque and not counted as a failure.**

What was seen that made it necessary: 0.5.63 marked the opaque answer of a `no-cors` request but not the one of a `redirect: manual` request, which the browser also answers with status 0; it was left as a known defect in that entry.

Now a capture whose answer type is `opaque` or `opaqueredirect` is shown as `opaque (no-cors: its status cannot be read)` by `click` and `payloads` and is skipped by the `Problems:` line. A request that got no answer still shows `0 (no answer)`.

How it was checked: a test in a real Chrome that failed before (a `fetch` with `redirect: 'manual'` to a path that answers 302 was shown as `no answer`) and passes after; `go vet` and the suite with `-race` pass (exit code checked); `node --check page.js` is clean.

Checked on the real screen of acme (2026-10-31, with the reconnected MCP server): a `fetch` with `redirect: 'manual'` to a local server that answers 302 showed `GET /go opaque (no-cors: its status cannot be read)` in `click` and in `payloads`, with no `Problems:` line.

## 0.5.63 — 2026-10-31

**An opaque answer (a `no-cors` request to another origin) is shown as opaque and is not counted as a failure.**

What was seen that made it necessary: in the real check of 0.5.60, a request that did finish was counted by `payloads` as `Problems: 1 with no answer (status 0)`, because the browser gives status 0 to the opaque answer of a `no-cors` fetch.

Now the helper marks a capture whose answer type is `opaque`. `click` shows `GET /path opaque (no-cors: its status cannot be read)` instead of `0 (no answer)`, `payloads` shows the same instead of the status, and the `Problems:` line and the backend-log lookup skip it. A request that really got no answer still shows `0 (no answer)` and still counts. `tools/list` does not change.

How it was checked: a test in a real Chrome that failed before: a click whose fetch is `no-cors` to a second server says `opaque` and not `no answer`, and `payloads` has no `Problems:` line; the suite with `-race` passes (exit code checked); `node --check page.js` is clean.

Checked on the real screen of acme (2026-10-31), with the helper of this version served from a local static server (the running MCP server was still the previous build): a `no-cors` fetch to another local server showed `GET /x opaque (no-cors: its status cannot be read)`, and a fetch to a closed port still showed `GET /x 0 (no answer)`. Not checked: an `opaqueredirect` answer (a `redirect: manual` request) was not marked as opaque here; 0.5.64 does it.

## 0.5.62 — 2026-10-31

**`click` and `press` show the URL change with parameter names, never values.**

What was seen that made it necessary: in the real run of 0.5.61, clicking «Nueva Cotización` came back as `[BLOCKED: Cookie/query string data]`: the line `URL: <before> → <after>` still carried the whole query string, which 0.5.56 had removed only from `state`.

Now that line uses the same form as `state` (`/path ?name1,name2 #hash`). The change is still detected by comparing the full addresses, so a change of only a value is reported. `tools/list` does not change.

How it was checked: a test in a real Chrome that failed before (`/next?token=SECRET9&id=7#top` was shown whole) and passes after (`/next ?token,id`); the suite with `-race` passes (exit code checked, not only the last line); `node --check page.js` is clean.

Not checked: the other lines of the answers that can hold an address (alerts, console errors, the text of a dialog): they show what the page wrote and are not cleaned.

## 0.5.61 — 2026-10-31

**`page download {report:true}` lists the files the session saw appear; `code leftovers` counts the test rows left in a table.**

What was seen that made it necessary: twice in this work an export left a file with customer data in `~/Downloads`, and the warning to delete it was written by hand each time; and the rows a test leaves (`ZZ_`) were counted by hand with `query`.

Now `download {report:true}` answers the files that `download wait` reported this session, with their size and the folder, and says that the user can delete them (the server does not). `code leftovers {table, column, prefix?, tenant?}` answers `leftovers table.column starting with ZZ_: N rows` (a count, never a value; `_` and `%` of the prefix are not wildcards) and, when N is not 0, says that they are test rows to clean through the screen and to mention in the report. The prefix is 1 to 20 letters, digits, `_` or `-`; table and column are valid names only. Both only list and count: nothing is deleted. `code(help)` lists the new action; `tools/list` does not change.

How it was checked: 2 tests that failed before: the report is `none` at first and then lists a pdf and an xlsx (with the folder and no cell value); `leftovers` counts the 2 `ZZ_` rows of 5 without showing them, does not take `ZZXC` for a match, says 0 rows for another prefix, and refuses a table name with `;` and a prefix with a quote. The suite with `-race` passes.

Not checked: `leftovers` against MySQL on the real database (the `LIKE … ESCAPE '!'` is standard and was run on SQLite); a file that appeared before `arm` is not listed; the list is lost when the server restarts.

## 0.5.60 — 2026-10-31

**`click` and `press` say when a request was still pending; `payloads` opens with a summary of the failed requests.**

What was seen that made it necessary: `click` waits about 2.5 s for the answers. When a request was slower, it could report `No effect seen` and the agent took the click for dead. And in `payloads`, one failed request among ten is easy to miss: in the real session the console had `Cannot read properties of undefined` and nobody looked.

Now, if any request counted by the helper has not answered when `click` or `press` writes its answer, the answer has `Still pending: N request(s) had not answered when this was written (it waited about 2.5 s), so the effect may be incomplete. Next: page(payloads) later shows how they ended.`, and `No effect seen` is not said. `payloads` adds, under its first line, `Problems: 1 with 4xx, 1 with 5xx, 1 with no answer (status 0) of N captured` when any captured request (all of them, not only the shown ones) failed, and nothing when none did. It is information; nothing is refused. `tools/list` is unchanged.

How it was checked: 2 tests that failed before: in a real Chrome, a click whose request answers in time does not say pending, and with a request left counted by the helper a click with no effect says `Still pending` and not `No effect seen`; and the summary counts 4xx, 5xx and status 0 over all the captures and is absent without failures. The suite with `-race` passes.

Checked later on the real screen of acme (2026-10-31): a button added in memory called a local server that answers after 8 s; `click` answered `Still pending: 1 request(s)…` and not `No effect seen`. The request ended as status 0 (a `no-cors` fetch has an opaque answer, whose status is 0), so `payloads` counted it as «no answer»: a `no-cors` request is shown as a failure. Not checked: a slow request of the app itself; the 2.5 s is the helper's, not configurable.

## 0.5.59 — 2026-10-31

**`page verify` takes `check METHOD address` as evidence: what `check` found about the stored row.**

What was seen that made it necessary: until 0.5.58, `verify` could back a claim only with a captured request or a downloaded file, so «the row was saved as sent» could not be tied to what the database said, which is the thing `check` already finds.

Now `check` tells the page tool, after each comparison, how many fields it compared, how many were different and how many only equivalent (`empty~`, `space~`), by the request it compared (`POST /api/quotes`). `verify {claim, evidence: "check POST /api/quotes"}` answers `OBSERVADO` when the last check of that request found no difference, `NO VERIFICADO` when it found some (`check found N differ of M fields`) or none was run, and `INFERIDO` when the fields were only equivalent or an interceptor was seen since `load`. `check` does not change: its answer and its arguments are the same, and `tools/list` is unchanged (1431 bytes). `query` is not accepted as evidence: a count or a row says nothing about which claim it backs.

How it was checked: 2 tests that failed before (they did not compile): `check` notes 4 fields and 0 differences for an equal row and a difference for another row; `verify` gives each of the five states above, including an interceptor. `go vet` and the whole suite with `-race` pass.

Not checked: on the real screen with a real `check`; a check of the same request run for two different rows counts only the last one; the note lives in memory and is lost when the server restarts.

## 0.5.58 — 2026-10-31

**`WEBSCOPE_LOG`: only the error lines written since the click or key began, not the last ones of the file.**

What was seen that made it necessary: checked on the real screen of acme (a 500 forced with `simulate`), the three lines that 0.5.57 added were errors from the day before and from hours earlier, with no relation to the request: an agent could blame its click for an old error.

Now `click` and `press` ask the listener to note where the log file ends (`/logmark`, which answers the offset) before the action, and the lines added afterwards come from the bytes after that offset (`/log?from=N`); `payloads` uses the offset of the last click or key. No clock and no timestamp format are needed, so it works for any log. Without a mark (`payloads` before any click), or if the file was rotated (it is shorter than the mark), it reads the end of the file as before. Nothing else changes: up to 3 lines, 200 characters each, only on status 0 or 5xx.

How it was checked: a test in a real Chrome that failed before (it showed the old error line): the log has an old error before the click, the failing request writes a new one, and `click` and `payloads` show the new line and not the old one nor the normal lines. The first version of the change deadlocked `payloads` (it held the lock that `logLines` took); the test caught it (timeout) and it is fixed. `go vet` and the whole suite with `-race` pass.

Not checked: on the real screen (needs one more reconnect); a backend that writes its error to the log after the answer reaches the screen (late lines are missed: the helper reads the log about 550 ms after the click); a log written by another process between the mark and the click, which counts as caused by the click; several clicks at once.

## 0.5.57 — 2026-10-31

**`WEBSCOPE_LOG`: when a request ends with status 0 or 5xx, `click`, `press` and `payloads` add the last error lines of the backend log.**

What was seen that made it necessary: in the session of 0.5.53, a request with status 0 was a fatal PHP error, and it took several steps to see it because the log was somewhere else.

Now, with `WEBSCOPE_LOG` set to a file, a click or key that caused a request with no answer (status 0) or a 5xx adds `Backend log (the last error lines of WEBSCOPE_LOG, as the log wrote them):` and up to 3 lines (the last ones that contain fatal, exception, error, panic, stack trace or uncaught; 200 characters each; read from the last 64 KiB of the file). `payloads` adds the same when any request it shows failed. The helper asks the local listener for the lines on a new route, `/log`, which needs the token like the others. Without `WEBSCOPE_LOG`, or without an error line, nothing is added and nothing fails. `tools/list` is unchanged.

How it was checked: 2 tests that failed before (they did not compile). In a real Chrome with a log file: a click whose request answers 200 brings no log, a click whose request answers 500 brings the fatal-error line and not the normal lines, and `payloads` brings it too; with no log configured the lines are empty. `go vet` and the whole suite with `-race` pass.

Not checked: a real PHP log (the format of each project differs; the match is by words, and a line that says «error» without being one shows); a log that rotates between the error and the click; a log on another machine; the lines are the log's own text and can hold data (a SQL with values, an address with a token): they are cut to 200 characters and the README says so, but they are not cleaned.

## 0.5.56 — 2026-10-31

**`page state`: the URL line carries the names of the query parameters, never their values; a 404 screen is said on the first line; over the limit it drops whole lines instead of cutting one.**

What was seen that made it necessary: in the session of 0.5.53, the answer of `state` reached the agent as `URL…(+1043 chars)`: the output of the Chrome tool was cut at the line that began with the URL. A URL with a query string can hold a token, and it is what output filters cut. And with a limit, `state` could stop in the middle of a string (`Title: A title that is long enough to be cut in the m…(+81 chars)`).

Now the line is `URL: /path ?name1,name2 #hash` (the hash is kept without its own query), a title or a heading (`h1` to `h3`) that says 404 or not found puts `Looks like a 404 page of the app … Next: open the right address.` first, and when the answer goes over `max` (3000 characters by default) the lines that do not fit are dropped and the last line says how many (`(… N more lines not shown: raise max)`). A line longer than `max` that is alone is still cut. The format stays text, one fact per line: the plan said JSON with fixed fields, but the text is shorter in tokens and the agents' flows already read it; what cut the answers was the URL and the limit, and both are fixed here.

How it was checked: 2 tests in a real Chrome that failed before (the URL line showed `token=SECRET9`, a 404 title was not said, and the limit cut the title in the middle) and pass after, with a normal screen not called a 404; the rest of the suite with `-race` still passes.

Not checked: that the Chrome tool no longer cuts the answer (the cause was inferred from where it cut; it needs one run on the real screen); `payloads` still shows the whole address of each request, with its query, because the agent needs it to tell requests apart; a hash that holds a router query (`#/p?id=3`) loses it; a legitimate screen that says «not found» in a heading is called a 404.

## 0.5.55 — 2026-10-31

**`page verify {claim, evidence}`: a table of the claims of a report, with a state that the server decides.**

What was seen that made it necessary: the same session as 0.5.53. The final report said what the agent believed, with nothing tying each sentence to something seen.

Now `verify {claim, evidence?}` adds a claim and answers `n. STATE claim`. `evidence` is a reference: `capture METHOD address` (a request `page` captured; the match is the one of `parity`) or `download` (a file that `download wait` found). The state is `OBSERVADO` when the reference exists, `INFERIDO` when a capture exists but an interceptor was seen on the page since the last `load` (the label of 0.5.53 stays on for the rest of that load), and `NO VERIFICADO` when there is no evidence, no request matches, no file was found, or the text is not a reference (free text never counts). The caller cannot set the state. `verify {report:true}` prints the table (`n. STATE claim (evidence: …)`), `{clear:true}` empties it; up to 50 claims of 200 characters. It is answered by the server, optional, and blocks nothing. `tools/list` is unchanged.

How it was checked: 3 tests that failed before (they did not compile) and pass after: a real capture is OBSERVADO, a missing capture, free text and `download` without a found file are NO VERIFICADO, a found file is OBSERVADO, an interceptor downgrades a capture to INFERIDO, and the report lists a claim without evidence as NO VERIFICADO and `clear` empties it. `go vet` and the whole suite with `-race` pass.

Not checked: a session where an agent builds its report from the table; whether `capture` is enough evidence for effects that only the database shows (that is `check` and `query`, whose results `verify` does not see); the table lives in memory and is lost when the server restarts.

## 0.5.54 — 2026-10-31

**`page download`: `{arm:true}` before a real click and `{wait:true}` after it, to prove that a file was downloaded.**

What was seen that made it necessary: the same session as 0.5.53. An export was reported as downloaded after checking only the HTTP answer and the bytes of a file built by the agent; no file had reached the downloads folder.

Now `download {arm:true}` notes the files already in the downloads folder (`WEBSCOPE_DOWNLOADS`, else `~/Downloads`) and `download {wait:true, seconds?}` (10 s by default, at most 60) waits for a new or changed file that stops growing, ignoring `.crdownload`, `.tmp`, `.part` and hidden files. It answers the name, the size, the type by the first bytes (`PK` plus `xl/workbook.xml` is xlsx, `word/document.xml` is docx, `%PDF` is pdf, plain text, or unknown) and the time; for an xlsx also the header names (first row, at most 30, 40 characters each) and the row count. No value of any cell is shown. With no file in time the answer is `Failed: no file appeared in <folder> within N s, so nothing was downloaded…`, marked as an error. It is answered by the server (no snippet) and it is optional: nothing else changes. `page(help)` lists it; `tools/list` is unchanged.

How it was checked: 4 tests that failed before (they did not compile without the action) and pass after, with an xlsx built in the test: the new file with headers and rows and without the cell value, a file that was there before `arm` is not reported, the type comes from the bytes and not from the name, a `.crdownload` alone fails after waiting the limit, and `wait` without `arm` fails. `go vet` and the whole suite with `-race` pass.

Not checked: a download through the real Chrome (the folder Chrome saves to may not be `~/Downloads`: set `WEBSCOPE_DOWNLOADS`); a file that Chrome renames after it ends (`file (1).xlsx` is a new file and is reported as such); xlsx whose first sheet is not `sheet1` by name sorting; `.xls` and `.csv` get only the type.

## 0.5.53 — 2026-10-31

**`page` marks a result as seen through an interceptor when `fetch`, `XMLHttpRequest.send` or the anchor `click` were replaced after the helper loaded.**

What was seen that made it necessary: in one session an agent reported «the export downloads the Excel» when no file had been downloaded. It had swapped the real click for a JavaScript interceptor and checked only the HTTP answer and the content of the file; nothing in the answers of `page` said that the flow was not the real one.

Now the helper keeps the three functions as they are once it is in place and compares them on every action and every 400 ms. When any differs, the answers of every action that goes through the page (`state`, `click`, `fill`, …) start with «Interceptor: <names> replaced after the helper loaded. What follows is what the page did with that patch, not what a real click did: do not report it as a check of the real flow.», and `payloads` starts with the same kind of line (the helper tells the listener by a new route, `/tamper`). It is a label, not a block: nothing is refused, and with no replacement nothing is added. If the patch is undone the label goes away.

How it was checked: a test in a real Chrome (`TestAnInterceptorPutOnTheFetchIsMarkedInStateAndPayloads`) replaces `fetch` after `load`; it failed before the change (no mark in `state` or `payloads`) and passes after, and `state` before the replacement has no mark. `go vet`, the whole suite with `-race` and `tools/list` (1431 bytes) were run.

Not checked: a page that replaces `fetch` before the helper loads (it is taken as the page's own, not marked); `XMLHttpRequest.open` and the constructor, which legacy pages replace themselves; a patch that is put and undone between two ticks of 400 ms; a real agent session. The state of evidence (`verify`) and the download action are still planned (ROADMAP, «Informes honestos»).

## 0.5.52 — 2026-10-30

**`code snap` takes a key of several columns, and `page load` says when the page still holds the helper of an older build.**

What was seen that made it necessary: in the acme sessions of 2026-10-30, 7 `snap` calls failed because the row is identified by several columns (`COTIZACION_ID,CAMPANA_ID,CLIENTE_ID`) and `key` took one. And while checking 0.5.51 on the real screens, `page(load)` run again after a rebuild did nothing in a tab that already had the helper (it installs once), so the tab kept answering with the old code and the new behaviour could not be told from the old one until the page was reloaded.

Now `key` is one column or several separated by commas; the key of a row is their values joined with `|` («changed 1|a: …»), a column of the key that does not exist is named, and a repeated key says to use several columns. The helper carries a version (the first 8 hex digits of the hash of `page.js`); the line that `page(load)` returns stops with «Failed: an older helper is in this page: reload the page, then page(load)» when the page holds a helper of another version, and loads as before otherwise. The line is longer (about 110 bytes).

How it was checked:
- Tests written first and failing: a key `id` that repeats fails, `id, part` works, the diff names rows as `1|a`, a missing column of the key is named; the bootstrap checks the version, passes `v=` and `page.js` records it. The suite passes with `-race`; `go vet` is clean.
- Run after `/mcp` against acme: `snap` with `key: "id_tarifario,id_servicio,id_variacion"` on `app_tipo_tarifario_servicios` (tenant demo) saved 103 rows and `snap_diff` said no changes. In a real tab the helper recorded its version, and with another version set the bootstrap answered «Failed: an older helper is in this page…» (a helper of an older build was simulated by changing the recorded version; no real older helper was left in a tab).
- Not checked: `snap_diff` after a real change with a composite key outside the test database. Not done: running `load` and an action in one step, because the helper is lost at every navigation and the call has to reach the browser through `javascript_tool` anyway.

## 0.5.51 — 2026-10-30

**`page shape` names a field by what the screen shows, leaves out the options of a list and the pagination, and `parity` ignores accents and no longer merges «Flete 1» with «Flete 2».**

What was seen that made it necessary: in the real Tarifario de ventas (legacy against new) the legacy listed fields by their technical name (`lstPaisOrigen_tar_BC`, `cnt_txtVigencia_tar_BC`) next to the labelled ones, nine options of a select2 dropdown as if they were buttons, and the new screen listed «Primera página», «Página siguiente» and a Vuetify field named «Open». 0.5.50 would also have merged «Flete 1» and «Flete 2» into one name.

Now, in `page.js`, a field whose label is only its name or id takes the first line of the text around it when that is 40 characters or less; buttons inside a list of options (`.select2-results`, `[role=listbox]`, `[role=option]`, `.v-list`) or a pagination (`.v-pagination`, `.v-data-table-footer`, `[role=navigation]`) are left out; a field named «Open» is left out. In `parity`, names are compared without accents («Pais» is «País»), and only an id at the end counts as a data row: three digits or more («Ruta 1004»), or any digits after `-`, `_` or `#` («V-2»). «Flete 1» and «Flete 2» stay two names.

How it was checked:
- Test written first and failing: accents and case match, a long row id is one name, and a missing «Flete 2» still shows. 153 tests pass with `-race`; `go vet` is clean.
- Run against the real Tarifario de ventas (IMPO AIR) after `/mcp`: the legacy went from 21 to 12 buttons (the nine options of the select are gone) and the technical names became «Pais Origen ×2», «Ciudad Origen ×2» and so on; `Tarifario` and `Proveedor` still pair with the new screen (`changes` lists them) and `Open` is gone. A first run of this version removed every field with the aria-label «Open», including `Tarifario` and `Proveedor`; the real screens showed it and the filter now looks at the field's own label.
- Not checked: the fields of the new form are «only in legacy» because the dialog was closed in that capture, which is a difference of state and not of screens. The column «LISTA PRECIOS» against «Lista de precios» stays unmatched (the words differ, not the accents). The text around a field is a guess: a layout where the first line of the parent is not its label gives a wrong name. `page.js` has no automated test (it needs Chrome).

## 0.5.50 — 2026-10-30

**`page parity` treats data rows as one name, groups repeated changes and skips labels made only of digits.**

What was seen that made it necessary: run against the real Tarifario de ventas (legacy against new, IMPO AIR), the new screen listed 10 fields «Seleccionar la ruta V-N» and 10 buttons «V-N», one per table row, which are data and not structure; `Proveedor: select-one → text` came out twice; the legacy listed the fields «1», «2», «3», «4» (unlabelled inputs).

Now the digits at the end of a name count as `#` when names are matched and shown, so «V-2» on one side and «V-1» on the other are the same name, and ten of them are one line «V-# ×10». A change of kind that repeats is one line with ×N. A field whose label is only digits is left out. Without those cases the answer is the same as in 0.5.49.

How it was checked:
- Test written first and failing: three buttons V-1..V-3 against V-7 are one name on both sides; digit labels are skipped; a repeated change is one line with ×2. 153 tests pass with `-race`; `go vet` is clean.
- Run against the real Tarifario de ventas (IMPO AIR, legacy and new) after `/mcp`, with `kinds: "field,column"`: «only in new» went from 24 lines to 6 (`Seleccionar la ruta V-# ×10` is one line), `Proveedor: select-one → text ×2` is one line, and the fields «1» to «4» are gone.
- Limit: «Flete 1» and «Flete 2» are also one name, so a missing «Flete 2» is hidden when «Flete 1» exists. Matching by the visible label of a technical name (`lstPaisOrigen_tar_BC`) is still not done.

## 0.5.49 — 2026-10-30

**Failures of `code` say what to do: `legacy` lists the folders that exist, a missing table says to pass `tenant`, and a call with `help:true` or `args` as JSON text is read.**

What was seen that made it necessary: in the acme sessions of 2026-10-30, 3 of 6 `code legacy` calls and 12 of 39 `code fields` calls failed. `legacy` said `no folder "…/tipo_tarifario" in WEBSCOPE_LEGACY` without saying which folders exist; `fields` without `tenant` read the central database and answered `Table 'acme.app_tarifario' doesn't exist`. Two agents also called `code {help: "true", topic: "migration"}` and `args` as a JSON text, and got `unknown action ""`.

Now `legacy` with a missing folder adds the first 15 folders of the nearest parent that exists. A «table doesn't exist» error (1146) of `fields`, `snap` or `rows` called without `tenant` adds «pass tenant to read the one of the app». A call with no action and `help` set to `true` is the action `help`, and `args` given as a text that holds an object is read as that object (`page` takes the same call, so it gets both too). A text that is not an object is still refused.

How it was checked:
- Tests written first and failing: the missing folder lists its siblings (and the root's folders when no parent exists); the hint appears only without `tenant`; `help` flag and `args` as text are read. The full suite passes with `-race`; `go vet` is clean.
- Not checked: against the real acme legacy folder, nor that an agent now gets the call right at the first try. `snap` with a composite key is not done.

## 0.5.48 — 2026-10-30

**`page parity {a, b, kinds?}` shows a repeated name once with ×N, and `kinds` limits what is compared.**

What was seen that made it necessary: in the parity of the CRM tariff screens, «only in legacy» listed 59 lines, almost all the same `chk_id[]` checkbox repeated and the step buttons of a four-step wizard, so the agent had to read the answer by hand to find the real gaps (a missing filter, a select that became a combobox).

Now a name that repeats is printed once with its count («field chk_id[] ×8»), and `[]` at the end of a name is ignored when matching. `kinds` (a list of `field`, `button`, `column`, `tab`, `heading`) compares only those kinds; an unknown kind fails and lists the valid ones. Without `kinds` the answer is the same as before, except for the ×N.

How it was checked:
- Tests written first and failing: a repeated name gives one line with ×3; `kinds: "field"` leaves the buttons out; an unknown kind fails. The tests of the package pass with `-race`; `go vet` is clean.
- Not checked: matching by the visible label when the legacy name is technical (`lstPaisOrigen_tar_BC` against «País de origen»): it needs the label of each field from `page.js`, and is left for another version. Not run against the real acme screens.

## 0.5.47 — 2026-10-30

**`code help {topic?}`: the server tells the agent when and how to use its tools for a migration.**

What was seen that made it necessary: the user asked for parity between an old and a new system many times, and the sessions of the test bed never used `legacy`, `flows`, `cases`, `rows`, `page shape` or `page parity`. The steps and the choice of tool lived in skills and in the project's own `CLAUDE.md`, which a server does not deliver: a session in another project had neither (its `CLAUDE.md` still named the tools of the old server, and the skills were only in this repository). The knowledge has to travel with the server. A second defect on the way: a failed `code` call ended with «Next: code(help) lists the actions», and `code` had no `help`.

Now `code help` lists the actions with what each is for, and `code help {topic: "migration"}` returns the eight steps of a migration (stories, legacy side, screens, requests, writes, cases, unit tests, report) with the tool of each and the limits of each tool; it is embedded in the binary. The instructions of the server add one sentence: for a migration, parity, user stories or test cases, call that first. The skills stay for projects that load them; the text is no longer only there.

How it was checked:
- Tests written first and failing (`undefined: codeActions`): help names every action; every listed action is known to the dispatcher; the playbook names only actions that exist (it caught «legacy code is» read as an action, so the text was reworded); the playbook stays under 4000 bytes; an unknown topic lists the known ones. 56 tests of the package pass with `-race`; `go vet` is clean.
- `tools/list` stays at 1431 bytes (the limit is 2500); the instructions of `initialize` grew to 1571.
- Not checked: that a model in a fresh session of another project reads the sentence and calls `help` when asked for parity; it needs a session after `/mcp`.

## 0.5.46 — 2026-10-29

**`code rows` leaves out the columns that pick the rows whatever their case.**

What was seen that made it necessary: the first run of 0.5.45 on the two `ZZ_` quotes of the test bed (one created by each system, both now `BORRADO`) listed `COTIZACION_ID` as a difference. It was asked with `cotizacion_id` and the database names it in capitals, and the name was compared as typed.

Now the comparison ignores case. The real run gave: 42 columns, 38 equal, 3 differ once the key is out (`MOTIVO_RECHAZO` empty on one side and filled on the other, `DESCRIPCION` with the same length and another text, `USUARIO` in another case) and the same count of child rows on both (1 log, 1 version, 6 general data, 10 instructions, 12 follow-up activities). It is what the by-hand comparison found.

How it was checked:
- A test written first and failing (`2 columns, 0 equal, 2 differ`), with a capitalised column asked in lowercase; 143 tests pass with `-race`, `go vet` is clean.
- Not checked: that the counts of the real run were read after the fix; they are the ones printed by 0.5.45, whose only error was the key column.

## 0.5.45 — 2026-10-29

**`code rows {table, a, b, children?, tenant?}`: how two rows of one table differ, without a value.**

What was seen that made it necessary: the write test on both systems (a `ZZ_` quote on each) was compared by hand with `query`, column by column, and the child rows were counted one table at a time. That found `MOTIVO_RECHAZO` filled by one system and empty in the other, and the case of `USUARIO`, but it is the step a person repeats for every action of every module.

`a` and `b` are `{column: value}` that pick one row each (more than one row is an error that asks for another column). The answer is the count of columns compared, equal and different, then one line per differing column with its class: `a filled, b empty`, `a NULL, b empty`, `case only`, `spaces only`, `same number, other text` (`7` and `007`), or `differs (a N chars, b M chars)`. The columns that pick the rows are not compared and no value is printed. `children` is `{child table: column}`: the rows of each table that carry the first value of `a` and of `b`, with `≠` when the counts differ. It only reads, in a read-only transaction; every name is checked as a plain identifier. The text of `tools/list` grew by about 60 bytes.

How it was checked:
- Tests written first and failing (`unknown action "rows"`): every class of difference, no value in the answer (`SECRET_NOTE` and the others are absent), the children counts with `≠`, two equal rows, and eight refusals (no table, a table with `;`, no `b`, a column with a backtick, no row, two rows, a bad child name, an unknown table). 51 tests of the package and 142 in all pass with `-race`; `go vet` is clean.
- Not checked: on the real quotes of the test bed (it needs `/mcp` to load this version); MySQL types other than text (dates and decimals are compared as the driver returns them as text, so `1.50` and `1.5` are «same number, other text»).

## Note on 0.5.31 — 2026-10-29

The condition of the skill `parity-tests` (the `tester` agent writes tests of a real service that fail on a planted regression) was met in part. On the quotes module of the test bed the agent wrote two tests of `QuoteService::isApprovable` for a rule the legacy code has (`grabar_aprobar_cotiza.php:70-72`: no approval without a client): an empty and a null client give `false`. Both passed, and both failed with the check inverted (`Failed asserting that true is false`); the code was restored and its `git diff` was empty. Checked by hand afterwards.

Limits: the tests use no database, because the `.env` of that project points at a live demo tenant and `phpunit.xml` has no test connection. So the happy case of approving and `create()` (which builds `DocumentSequenceService` inside the method) are not covered. A client of only spaces is left open: the legacy compares `== ""`, the new code rejects it (`filled('  ')` is false); whether the legacy trims the value first is not known.

## 0.5.44 — 2026-10-29

**The columns section of `code legacy` tells a column that is written from one that is only read.**

What was seen that made it necessary: in 0.5.43 a column counted as named when a backend file had it between quotes, so a `'estado' => $this->estado` in a Resource, a validation rule or a `where('estado', …)` hid a column that nothing stores. A false «named» is a missed gap, and the user asked for precision first.

Now three groups: «named nowhere in the backend»; «named only where it is read, never assigned or filled» (a Resource, a Request, a filter); and «written in the backend, but not in the module's folders». A column is written when it is assigned: `'col' =>` in a file that is not a Resource, Request or Collection, `->col =` (also `.=`, `+=`, `-=`, not `==`), or a name in a `$fillable` or `$guarded` list. A column written inside the module's folders is in none of the groups.

How it was checked:
- The test has a column per case: one in a Resource (`solo_lectura`), one in a Request (`en_request`), one in a `where` (`filtro_col`), one in `$fillable` (`rellena`, in none of the groups), one written in a shared folder (`usuario`), one named nowhere. It failed first (the new groups did not exist), then passed; 48 tests pass with `-race`, `go vet` is clean.
- Real quotes legacy: 267 columns on 18 tables, and the three groups are empty, so every column the legacy writes is assigned somewhere in the module's folders.
- Not checked: a write by a path the rules do not see (`DB::table()->insert($data)` with an array built elsewhere, `fill($request->all())`); then the column would show as «only read». A count by hand of which of the 267 are assigned by each rule.

## 0.5.43 — 2026-10-29

**The columns section of `code legacy` splits what it finds in two groups, so a doubtful column is never reported as a gap.**

What was seen that made it necessary: in 0.5.42 a column counted as named when any backend file had it, so common names (`estado`, `usuario`) hid real gaps; naming only the module's files would swap that for false gaps, since shared code outside the module can write a column too.

Now the section prints `checked N columns on M tables; module folders: …` and two groups: «named nowhere in the backend» (the sure ones) and «named in the backend but not in the module's folders (who writes them?)» (to check by hand). The module folders are the `app/Modules/Area/Module/` of the controllers, services and models that `map_back` finds for the module. A name still counts when a read names it.

How it was checked:
- The test now has the three cases: a column named in the module, one named only in a shared folder (`usuario`) and one named nowhere (`chk_flete`, with `chk_flete_nuevo` in the module not hiding it). It was written together with the code, not failing first; it fails on 0.5.42 because the headings do not exist. 48 tests pass with `-race`, `go vet` is clean.
- Real quotes legacy: 267 columns on 18 shared tables, both groups empty, with 20 module folders (the services import models of other modules, so «the module» is wide).
- Not checked: whether the new code writes a column (a read names it too); the `chk_*` flags, which are PHP variables.

## 0.5.42 — 2026-10-29

**`code legacy` reads the columns of SQL that PHP builds by concatenation, and says how many it checked.**

What was seen that made it necessary: the first run of 0.5.41 on the real quotes legacy answered `-`, and the 0.5.41 entry below read that as «the legacy writes through stored procedures». That was wrong. The SQL is inline, in `procedimientos/cotizaciones.php` (`INSERT  INTO  app_cotizaciones  (cotizacion_id, …," .` then the rest of the list on the next line as another string); the parser could not read a list cut by `" . "`.

Now a `" . "` between two strings is read as a space, so a list or a `SET` built in pieces is read whole. The section also prints `checked N columns on M tables`, so an empty answer says what it covered.

How it was checked:
- The test of 0.5.41 grew an `INSERT` and an `UPDATE … SET` split by concatenation and failed (`otra_col`, `campo_union` lost), then passed; 48 tests pass with `-race`, `go vet` is clean.
- Real quotes legacy: the file above gives 33 tables and about 400 columns (`app_cotizaciones` 32 columns, among them `motivo_rechazo`). Every one of the 32 is named somewhere in the backend, so the section is empty for a true reason, but the test is loose: a name such as `estado` or `usuario` is named by any file.
- Not checked: that a column is written by the new module's own services (it counts a name anywhere in the backend); the `chk_*` flags, which are PHP variables of a call and not columns in the file.

## 0.5.41 — 2026-10-29

**`code legacy {path, module}` lists the columns the legacy writes that the new backend never names.**

What was seen that made it necessary: when the old and the new system share one database, a column only the legacy writes is lost without a warning, and until now it was found only if a test action happened to touch it (the seven `chk_*` flags of the quotes were found by hand). `legacy` compared tables, not columns.

Now the answer of `legacy` with `module` ends with a section «columns the legacy writes and the backend never names», one line per shared table. The columns come from the SQL of the legacy files: the list of an `INSERT`, the names assigned by `UPDATE … SET` and by `ON DUPLICATE KEY UPDATE`, comments skipped and names built from a PHP variable left out. A column counts as named when any PHP file of `WEBSCOPE_BACKEND` has it between quotes or after `->`, case apart. It prints names, never values; without `WEBSCOPE_BACKEND` it says it did not compare.

How it was checked:
- A test written first and failing: an `INSERT` spread over lines, an `UPDATE … SET`, a commented statement, a variable in the list and a table only the legacy has; the listed columns are exactly the ones the backend does not name, and `chk_flete_nuevo` in the backend does not hide `chk_flete`.
- `go vet` and `go test -race ./internal/code` pass (48 tests).
- Run on the real quotes legacy: the section answered `-` (see 0.5.42: the parser missed SQL split by `" . "`).
- Not checked: a legacy that writes with inline SQL beyond the fixture.

## 0.5.40 — 2026-10-29

**`page fill` warns when a name fits more than one field.**

What was seen that made it necessary: on the legacy quotes screen the list filters and the form below them share labels (`ORIGEN`, `DESTINO`, `Vendedor`, `Nombre del cliente`). `fill {"ORIGEN": …}` answered `ORIGEN: ok` and had written into the list filter, not into the form, which showed `...` and was found by reading the values one by one. A dialog's field is already chosen before the page's; a page with two panels has no dialog.

Now, when the first of the three ways of finding a field that gives a result (exact label, name, id or placeholder, label that contains the text) gives more than one, `fill` still fills the first (a dialog's before the page's, as before) and the line of that field ends with `(warning: 2 fields answer to this name; filled #e1, the others are #e2. Name one by its id to be sure)`, naming up to three of the others by id, `[name=…]` or label. A name that fits one field answers as before, and an id still picks one without a warning.

How it was checked:
- A real-Chrome test written first and failing (`Estado: ok` and nothing else): two selects labelled `Estado` give the warning with both ids; `fill {"e2": "dos"}` gives `e2: ok` with no warning; a name that fits one field (`Nombre`) gives none.
- `go test -race ./...` passes (138 tests); the existing fill tests (the `Nota` in a dialog and in the page) still pass because the line still starts with `Nota: ok`.
- Not checked: on the legacy screen itself (it needs `/mcp` and a tab); a name that matches fields in a hidden panel (hidden fields are not counted).

## 0.5.39 — 2026-10-29

**`page shape {name, selector?}`: the shape of one part of the screen.**

What was seen that made it necessary: on the real quotes screens the legacy is one page («Pantalla Cotización») with the list and a 12-step form, and the new front splits them, so the shape of the whole screen gave 34 fields, 19 buttons and 12 tabs against 14, 13 and none, and `parity` was almost all noise.

`selector` is a CSS selector, like the one `audit` and `read` take. Fields, buttons, table columns, tabs and headings are then taken only from inside the first element it matches, so the list of the legacy and the list of the new front can be saved and compared without the form. A selector that matches nothing answers `Failed: nothing matches …` and one that is not a selector answers `Failed: bad selector …`; neither saves anything, so an old shape of the same name is not replaced by an empty one. Without `selector` it behaves as before.

How it was checked:
- A test in a real Chrome written first and failing (`shape «none» saved: 8 fields` for a selector that matched nothing): the whole page gives its eight fields; `#clean` gives one field (`Nombre`) and one button (`Enviar`) and no headings; a selector that matches nothing and one that is not a selector both fail and save nothing.
- `go test -race ./...` passes (137 tests).
- Real run on the quotes list (legacy `COTIZA_NEW` on `demo.localhost:8088`, new front on `demo.localhost:5173`, read only, tabs closed afterwards): `div.ibox:has(#grid_tabla)` on the legacy gave 16 fields, 1 button and 10 columns (the whole page: 34 fields, 19 buttons, 12 tabs) and `.v-card-text:has(table)` on the new front gave 14 fields, 12 buttons and 9 columns. `parity` then listed only filters, columns and pager.

What the real comparison says (evidence; a person decides):
- **The list filters have a twin in nearly every case, under another name.** `Estado` / `Filtrar por Estado`, `Solicitud` / `Requerimiento`, `lstServicioBC` / `Servicio`, `lstIncotermBC` / `Filtrar por Incoterm`, `Buscar por` + `Información adicional` / `Campo adicional` + `Valor del campo`, `Fecha` / `Datepicker input`, `Elementos por pagina` / `Open`, and the legacy text fields for quote, client, origin, destination, advisor and DO / the one search box («cotización, cliente, contacto, asesor, campaña, ruta o DO»). The new list adds filters by subtype, office and currency.
- **Sorting.** The legacy has `Ordenar por` and `sentido`; the new columns (Cotización, Cliente, Vendedor, Fecha, Subtipo, Estado, Moneda, Ruta) are sortable by their header, and no sort parameter went in the list request because nobody sorted. Not clicked.
- **Columns.** The legacy has `REQ` and `DO ASOCIADO`; the new list has no such columns and adds `Subtipo`, `Moneda`, `Ruta` and `Acciones`. Where the new list shows the requirement and the DO was not checked.
- **Detail.** `POST cotiza_frm.php` (`cliente_activo`) against `GET /api/quotes/071`: with a map of 27 renamed pairs (`cotizacion_id`→`quote.id`, `fecha_ingreso`→`quote.entry_date`, `do_asociado`→`quote.delivery_order_id`, and so on; the pairs are my reading of the names, not confirmed) the legacy-only fields are `PROFILE`, `archivo`, `cargo_contacto`, the seven `chk_*` flags and `consecutivo` (very likely `quote.consecutive`, not mapped). The new answer adds display names (`advisor_name`, `status_name`, …), `requirement_id`, `shipping_observations`, `created_at`, `updated_at` and `user_id`.
- **The seven `chk_*` flags** (`chk_origen`, `chk_flete`, `chk_seguro`, `chk_destino`, `chk_aduana`, `chk_terrestre`, `chk_condiciones`): in the backend only `Quote.php:65` names `chk_flete` (no read or write besides the model, by `code consumers`), and in the demo tenant none of the 64 quotes has a value in any of the seven. So here they are unused; in a tenant with real data they may not be.
- One click on the new row `071 v.1` made seven requests (`GET /api/quotes/071`, `list/all`, `list/users`, `quote-versions`, `quote-notes/071/SPA`, `quote-conditions/071/1`, `additional-info/quote/071`).
- Not checked: the sort, the write operations, the data in a tenant with real quotes, and whether `PROFILE`, `archivo` and `cargo_contacto` live in another endpoint of the new front.

## 0.5.38 — 2026-10-29

**`page parity {req_a, req_b}` reads the query string of a GET and no longer prints the values of the address. First real comparison of the old screen and the new one.**

What the first real run showed (legacy `comercial/MDI_CCIAL.php?enl_id=COTIZA_NEW` on `demo.localhost:8088` and the new list on `demo.localhost:5173`, two tabs, one session, read only, the same action on both: «Buscar» on the quotes list, default filter `ENPROCESO`):
- A leak I had promised not to have: the request line of `parity` printed the address with its query string (`…/api/quotes?page=1&per_page=10&search=07&filters…`), so the values a user typed (and anything else on an address, like a token) reached the agent. Now it shows the address without the query and the number of parameters (`demo.localhost:8001/api/quotes ?5 params`).
- A `GET` sends its fields in the address, and `parity` read only bodies, so the new side showed no sent fields at all. It now adds the names of the query parameters to the sent names (a list index becomes `[]`: `filters[statuses][]`), for both sides.

How it was checked:
- A test written first and failing: a legacy form `POST` and a modern `GET` with a query string; the values (`secretvalue`, `ENPROCESO`) must not appear anywhere, the line says `?5 params`, and with a `map` that renames three legacy names the only differences are `filters[with_requirement]` and `per_page`. The earlier tests (bodies only) still pass.
- `go test -race ./...` passes (136 tests).
- Real run before the fix, through the registered server: legacy `POST …/server/grid_ppal.php` against modern `GET /api/quotes`.

What the real comparison says (evidence, to be confirmed by a person; none of it is a verdict):
- **One search, two shapes.** The legacy list sends 17 form fields (`txtCotizaBC`, `txtNomClienteBC`, `txtOrigenBC`, `txtDestinoBC`, `txtNomAsesorBC`, `txtDOBC`, `txtFechaIniBC`, `lstEstadoCBC`, `lstRequestCBC`, `lstServicioBC`, `lstIncotermBC`, `lstInforden`, `lstInfordenValor`, `orden`, `sentido`, `itemsPerPage`, `page`); the new one sends `page`, `per_page`, `search`, `filters[statuses][]`, `filters[with_requirement]` and `filters[attribute_value]`. The new list has one search box for quote, client, contact, advisor, campaign, route or DO where the legacy has separate fields; it has no sort control in the request (the legacy sends `orden` and `sentido`).
- **The legacy search does more than a search.** One click on «Buscar» made four requests (`grid_ppal.php`, `cotiza_frm.php`, `preliquidacion.php`, `cotiza_listas.php`): after listing, it loads the form, the pre-settlement and the lists of quote `071`. The new list makes one.
- **Fields of the legacy row with no obvious twin in the new list row:** the seven `chk_*` flags (`chk_origen`, `chk_seguro`, `chk_flete`, `chk_destino`, `chk_aduana`, `chk_terrestre`, `chk_condiciones`), `archivo` and `PROFILE`. The money fields have twins (`compra`, `venta`, `profit` and `purchase_total`, `sales_total`, `profit`). The names are otherwise different in style (Spanish column names against English snake case), so a `map` is needed to compare.
- **A legacy defect, seen and not fixed:** the answer of `server/preliquidacion.php` carries the SQL it ran (`"sql":["SELECT a.* … FROM app_preliquidacion a INNER JOIN …"]`) in its JSON. The new list answer carries no SQL.
- **Screen shape.** `page shape` and `page parity {a, b}` on the two screens gave 34 fields, 19 buttons and 12 tabs against 14, 13 and none, almost all noise: the legacy is one page («Pantalla Cotización») with the list and a 12-step form, and the new front splits them, so the names do not pair. The shape of a part of the screen (a `selector`) would help; it is not built.
- **The table view** `viewc_lstprovees_ope` (read by the legacy supplier list of the pre-settlement) does not exist in the tenant database (`Error 1146`).
- Not checked: a map built from `code fields`, the answer of the detail (`cotiza_frm.php` against `GET /api/quotes/{id}`), the write operations (only reads were done).

## 0.5.37 — 2026-10-29

**`code legacy {module}`: the tables only the legacy has are split into «named elsewhere in the backend» and «not named in the backend».**

What the real run showed (the first one through the registered server, with `WEBSCOPE_LEGACY` and the two-origin `WEBSCOPE_APP_URL` set): `crm/quotes` against the legacy `cotizadorX_new` listed 22 tables as `only in legacy`. A search of the backend for each one found 21 of them named in other PHP files (`tm_awb` in 18 files, `tm_ordenes_infogral` in 21, `tm_bl` in 16, `tm_actividades_oper` in 17, `app_wp_message` in 5, among others) and only `viewc_lstprovees_ope` in none. The 21 are not missing: the new module reaches them through services and models that `flows` does not follow (it reads the models that a controller or a service imports). Reporting all 22 as a possible gap was wrong in 21 cases.

Now, with `WEBSCOPE_BACKEND` set, the answer has two lines instead of one:
- `only in legacy and named elsewhere in the backend:` each table with the number of files that name it (`tm_awb (18 files)`);
- `only in legacy and not named in the backend:` the candidates for a real gap.

A name counts if it appears as a whole word in any `.php` file of the backend (comments and strings too: it is a text search, so a table named only in a comment also counts as named). Without `WEBSCOPE_BACKEND` the answer keeps the old single line. Each list stops at 60 names with a count of the rest.

On the real module the only candidate is `viewc_lstprovees_ope`, a database view that the legacy file `server/preliquidacion_provees_lst.php` reads (`SELECT cedula,nombre,tipo FROM viewc_lstprovees_ope WHERE tipo = …`, the list of suppliers by type for the pre-settlement). No backend file and no migration of either project names it. This is evidence for a question, not a verdict: the new pre-settlement may take its suppliers from `tm_proveedores` (a table both sides use), and nobody has compared the two lists.

How it was checked:
- A test written first and failing (`only in legacy:` was one line): a backend file that names two of the five legacy-only tables, in code and in a comment; the answer gives those two with `(1 file)` and the other three as not named. A test with no `WEBSCOPE_BACKEND` and one with 400 tables, which must keep both lists under the size limit with `(+N more)`.
- `go test -race ./...` passes.
- Real module, read only: the split above (21 named elsewhere, 1 not).
- Not checked: that each of the 21 is used by the flow of the new quotes (a name in the backend is not a flow); the view in the database itself (`SHOW CREATE VIEW` was not run).

## 0.5.36 — 2026-10-29

**`WEBSCOPE_APP_URL` accepts several addresses, separated by commas. The old app and the new one in one session.**

What was seen that made it necessary: in the app where this is tested the legacy answers on `http://localhost:8088` and the new front on `http://demo.localhost:5173`. The listener answered one origin, so a tab of the other one had every capture refused: `payloads` looked empty and `page parity {req_a, req_b}` (0.5.28) could not hold a request from each side.

What changes:
- Each address of the list is read as the browser writes its origin (lowercase, no default port, no path), blanks and duplicates are dropped, and the listener answers the `Origin` of any of them and no other. The `Access-Control-Allow-Origin` it sends is the one that asked.
- `load` carries the list in its `o` parameter (the one-line bootstrap stays under 450 bytes with three addresses); the helper in the page warns about a tab only when the tab's origin is in none of the list, and the warning names the list.
- `payloads` names the list when it says that a request from another origin was refused.
- A list with no usable address fails as before, with the same `WEBSCOPE_APP_URL is not set` text plus a hint about the list.
- One address works exactly as before. The captures of all the origins go to the same list (50 at most), so a legacy request and a modern request can be compared with `page parity {req_a, req_b}`; `page(load)` still has to be run again in each tab and after every navigation of the legacy, which reloads the page.

How it was checked:
- Three tests written first and failing (403 for every origin of a list, `WEBSCOPE_APP_URL is not set`): the listener answers each of three origins (one written with capitals and a path) and refuses a fourth, and `payloads` keeps all three requests and names the list; the bootstrap carries the three origins and stays short, and a list of blanks and a non-address is refused; and a real Chrome where the tab's origin is the second of the list (no warning) and is in none (the warning, with the list).
- `go test -race ./...` passes (134 tests).
- Not checked: with the real legacy and the real new front in two tabs at once (this needs your Chrome and `/mcp`); a legacy page that opens its own windows or frames at another origin.

## 0.5.35 — 2026-10-29

**New: `code legacy {path, module?}`. What the old PHP of a module calls, writes and reads, against what the new module touches.** (It is the version the roadmap called 0.5.29; the number moved on.)

`path` is a folder inside the new variable `WEBSCOPE_LEGACY` (for example `comercial/cotizadorX_new`). The answer has:
- `## endpoints`: each PHP file of the folder that some page or script calls, with the first caller as `file:line` and `(+N)` more, the tables its SQL writes (`W`) and the ones it only reads (`R`); a table written and read is listed once, as written. A caller is a quoted `.php` address in any `.php`, `.js`, `.html` or `.htm` file of the legacy: `folder/server/x.php` from anywhere, or `server/x.php` from a file of the folder itself.
- `no caller found:` the first six files nobody names (pages loaded by address, includes or dead code) and a count.
- With `module` (a front module such as `crm/quotes`): `## against «crm/quotes»` with `only in legacy`, `only in new` and `both`, comparing the tables of all the PHP of the legacy folder with the tables the new module's models name (the ones `flows` prints).

Safety: the path must stay inside `WEBSCOPE_LEGACY` (no `..`, no absolute path, no link that leads out), links are not followed, `.env*`, `vendor`, `node_modules`, `storage` and `.git` are not read, and the answer carries table names, never values. It reads PHP with the SQL inside the file and jQuery calls with a quoted address; other legacy styles will find less, and it says that it is an estimate. It lives in its own file, `internal/code/legacy.go`, like the other pieces tied to a stack.

How it was checked:
- A legacy tree of mentions and traps and six tests written first (they failed with `c.legacy undefined`): endpoints with callers from another folder and from inside the module, the writes, the reads, a table written and read listed once, a comment that says «update the user list from the form» that must not give a table `the`, a same-named file in another module, a `.env`, a `node_modules` and a link out of the root that must not leak; the comparison with the new module; the refusals (no path, `../`, absolute, missing, through a link, no `WEBSCOPE_LEGACY`); the comparison surviving 400 endpoints; and `ON DUPLICATE KEY UPDATE valor = 2` that must not give a table `valor`.
- `go test -race ./...` passes (131 tests); `tools/list` is still under its budget.
- On the real legacy of the app (`comercial/cotizadorX_new`, read only): 81 PHP files, 67 with a caller, the callers with their line, and the comparison with `crm/quotes`: 38 tables on both sides, 5 only in the new (`plantillas`, `app_lista_precios`, `app_lista_precios_detalle`, `app_tipo_cargos`, `tm_quote_links`) and the rest only in the legacy, among them `app_wp_message`, `sgg_consecutivos`, `app_campana_bitacora` and the import and export tables (`tm_importaciones`, `tm_exportaciones`, `tm_bl`, `tm_awb`, `tm_containers`).
- What the real run showed and was fixed here: the comparison was at the end and the 8000-byte cut removed it (the endpoints now take what is left); each line carried two long callers (now one and a count); and `ON DUPLICATE KEY UPDATE valor = …` made `id`, `iva` and `valor` look like tables.
- Not checked, and visible in the real answer: the `grabar_*.php` endpoints show no tables because their SQL is in an included file (`procedimientos/cotizaciones.php`, which does list its tables as `W`); the tables only in the legacy are not proof of a gap (the import and export tables may belong to another new module, and some names may come from SQL kept in strings that are not a query); `app_wp_message` and the others have not been confirmed as missing in the new module.

## 0.5.34 — 2026-10-29

**`flows` shares its endpoint room between tables, endpoints and warnings. Third run on `crm/quotes`.**

What the third run showed (0.5.33 confirmed: `cases` now sees every endpoint, 99 cases for the module, 79 shown before the byte limit, with the write endpoints of `quotes`, `quote-versions`, `quote-taxes` and `quote-conditions`):
- `flows` listed 40 `model … -> table …` lines (the module touches 40 tables, from `app_cotizaciones` to `app_trm`) and nothing else: the 91 lines that were cut were all the endpoints, the front addresses with no route and the services. Tables first had fixed the lost tables and moved the loss to the endpoints.

Now the room under `## endpoints` is shared: 3/10 for the tables, 2/10 for the services and the `api without a route in the backend` warnings, and the rest for the endpoints. Each part says what it cut (`… N more lines of models not shown`).

How it was checked:
- One test written first and failing (`tablesFirst` took no room): 100 tables, 20 endpoints, a warning and a service in 3000 bytes give the first table, the first endpoint in its short form, the warning, the note that tables were cut, tables before endpoints, and no more than 3300 bytes.
- `go test -race ./...` passes (127 tests).
- Real `crm/quotes` with this version: 11 tables (32 more cut), 21 endpoints in their short form (39 more cut), the 10 front addresses with no route, and a note of 17 more lines of services and warnings. Nothing is empty.
- Not checked: whether 3/10 is the right share for a module with few tables and many endpoints (the unused room of one part is not given to another).

## 0.5.33 — 2026-10-29

**`flows` and `cases` use every endpoint, and `flows` keeps the tables. Second real run on `crm/quotes`.**

What the second run showed (after the 0.5.32 fixes, which it confirmed: views first, statuses on top, `INS COTIZA`, required fields grouped, 67 cases with no cut):
- `cases` had only 10 `http error` rows for a module with about 60 endpoints. `map_back` cuts its own endpoint list to fit 8000 bytes («… and 49 more endpoints not shown») and `cases` read that shortened list. The `empty` row of every list endpoint and the error row of every write endpoint after the tenth were never produced.
- `flows` lost the `model … -> table …` lines (47 lines cut): they come after the endpoints, and the cut takes the end.

Now:
- `map_back` takes an internal `uncut` argument and `flows` and `cases` use it, so they see all the endpoints and share the room themselves. What `map_back` prints for a person is the same.
- `flows` puts the models with their table first, then the endpoints, then the rest, so a cut loses the last endpoints and not the tables. Each endpoint line is shortened to `QuoteController.php:234` instead of the whole path and the front file it is called from (about 40 bytes less per line); `map_back` keeps the long form.
- `cases` stops adding rows when the next one would not fit and says `(N cases, M shown: ask for a narrower module)` with the real N. Before it counted rows (150) and the 8000-byte cut came first, so the count line could be lost.

How it was checked:
- Two tests written first, against 400 extra endpoints on top of the fixture: `flows` keeps `-> table app_cotizaciones` and the short controller form (it failed at 8003 bytes with no table); `cases` reports at least 400 cases for 400 write endpoints (it stopped at 49 and lost the count). The test of long screens now expects the table line first under `## endpoints`.
- `go test -race ./...` passes (126 tests).
- Not checked: the real module with this version (the registered server has to be reconnected with `/mcp`); how many cases a module with hundreds of endpoints produces before the 8000-byte limit stops it (the rest is reported by count only).

## 0.5.32 — 2026-10-29

**`flows` and `cases` after the first run on the real quotes module (`crm/quotes`: 25 front files, 53 API calls, 60 endpoints).** Four defects the fixture tests could not show.

- **`flows` lost the endpoints.** The screens used the whole 8000 bytes and the answer was cut before `## endpoints`, so `cases` had no endpoint cases. Now the screens keep 5/9 of the room and the endpoints the rest, each cut on a line with a count of what was left out (`… 49 more lines of screens not shown`).
- **The main screens were the ones cut.** Files were listed in path order, so the 14 components came before `QuoteIndex.vue` and `QuoteShow.vue`. Screens under `/views/` now come first, and the module-level `statuses` line goes to the top.
- **`cases` was too wide.** It repeated the same two long sentences on each of its first 40 rows and filled 8000 bytes. The verification text is now said once, in a `Verify:` line, and the table has four columns (`id`, `flow`, `case`, `run with`); the same case is never listed twice (`rateType` and `minimum` appear twice in the pre-settlement form); and a label that is a translation key (`{{ $t('common.cancel') }}`) is not used to click, because no screen shows it written like that: the row says to click the button that calls the action. The required fields of one screen are one row (`required empty: qty, grossWeight, weightUnit, dimensionUnit`, «fill all but one of them at a time, save»), not one per field. The limit is 150 rows.
- **A permission lost half its name.** `can('INS', 'COTIZA')` was read as `INS`; it is now `INS COTIZA`.

`flows` and `cases` use `map` and `map_back` unchanged; `map` still cuts at 8000 bytes as before (a new internal function gives the others the uncut text).

What the first real run showed about the module (evidence, not verdicts):
- The main list `QuoteIndex.vue` has actions guarded by status (`item.status === 'APROBADA'`) and by permission (`INS`, `UPD`, `DEL`, `MIGR`, `D` on `COTIZA`).
- There are six statuses: `ENPROCESO`, `REQUERIMIENTO`, `APROBADA`, `DO_ASIGNADO`, `RECHAZADA`, `BORRADO`.
- Ten API addresses called by the front have no route in `app/Modules/**/Routes/*.php` (`characteristics/quote/{}`, `pieces/quote/{}`, `quote-notes/{}`, `quote-pre-settlements/table/{}`, the `files/cotizacion/proveedor` family and others). They may be defined elsewhere or be dead calls; not checked.

How it was checked:
- Four new tests, each written against the old behaviour: long screens still leave the endpoints in `flows`; `cases` keeps each case once, groups required fields and drops translation-key labels; views come before components with the statuses on top; a two-part permission keeps both parts.
- `go test -race ./...` passes (124 tests).
- Real quotes module through the registered server, twice (before and after the first three fixes): the endpoints appear and `cases` lists the `http error` rows with the right prefix (`quotes/approve/`).
- Not checked: the last two changes (views first, grouped required fields, two-part permission) on the real module; the row count after grouping; whether the ten addresses without a route are real defects.

## 0.5.31 — 2026-10-29

**New skill `.claude/skills/parity-tests/SKILL.md`. From «the legacy did X» to a plan of unit tests that holds the new code to it.** No Go code changes: what the tools print is the same; only the version number moves.

The comparison tools (`flows`, `parity`, `cases`) say what differs, not who is right; a unit test says it. The skill takes the confirmed stories and the `migration-parity` report and walks the agent through:
- one rule per finding, written as `Legacy: input → output. New: input → output`, with both sides from evidence, and anything deduced from a name marked `unconfirmed` and asked about;
- the lowest layer that owns the rule (service, request or validator, policy, and only then a screen case), found with `code map_back` and read before deciding;
- one case per behaviour: happy, the edge the legacy handled (empty, zero, maximum, a repeated value, `007`) and the refusal;
- whether each test must pass or fail today (a failing one is a confirmed regression), and never changing an expected value to make it pass without asking which side is right;
- following the project: the stack skill, two existing tests as a model, and asking before creating a test structure that does not exist;
- a hand-over to the `tester` agent, not writing and planning in the same pass;
- proof: run the tests, then plant one regression and see its test fail.

Rules it keeps: no row values in tests, the legacy side of a test is a fixed expected value taken from evidence and not a call to the legacy system, no tests on names alone.

What was seen that made it necessary: after the comparison the user wanted «to contrast functionalities» in unit tests, and nothing connected a finding to a test: each session decided by itself the layer, the cases and what to do with a test that failed on the new code.

How it was checked:
- `go vet` and `go test -race ./...` pass; the Go code did not change.
- The skill was read against the layers each stack skill requires (services, request classes, permissions) and against the output of `code cases` and `parity`; every tool and field it names exists.
- Not checked: the roadmap's condition for this version, that the `tester` agent writes tests of a real service from the skill and they fail on a planted regression. There was no real module in this session. The skill has never been run by an agent, so the steps may be too long or too short for one.

## 0.5.30 — 2026-10-29

(There is no 0.5.29: it is the reading of the legacy code, which waits for the answers about where the legacy lives.)

**New: `code cases {module}`. From what a module does to what has to be tested, as a table to run.**

It builds on `flows` and answers one row per case, five columns: `id`, `flow`, `case`, `run with`, `verify with`. The rows it makes:
- `happy: <action>`: run with `page click «text»`, verify with `page payloads` and `check` against the saved `ZZ_` row.
- `permission: <name>`: for each `can('…')` that guards an action or a part of the screen, `page state` as a user without it.
- `required empty: <field>`: for each field marked `*`, fill the rest, save, and look for the error and for no request.
- `status: A, B`: the statuses of the module, with a count by status in the base.
- `http error`: for each POST, PUT, PATCH or DELETE endpoint, `page simulate {match, status: 500}` and repeat the action; the id in the address becomes a prefix (`quotes/approve/`) so it matches the real request.
- `empty`: for each list endpoint (a GET with no id), a filter that matches nothing.

It runs nothing and says so in its first line: it is a list, not a result. Writes only with the user's approval and on `ZZ_` rows. At most 120 rows; it says how many exist. Without `WEBSCOPE_BACKEND` it gives only the screen cases.

**New skill `.claude/skills/migration-parity/SKILL.md`.** Chains the pieces for a migrated module: confirmed flows, the legacy side with `state` and `payloads`, screen `shape` and `parity`, request `parity` with a `map`, `check` of the saved row, then `cases`, adding one case for every difference found, and a report table that ends with what was not checked. It also lists what each tool cannot say (names are not behaviour; a rule in a service is found only by reading). Not tried with a real agent yet.

What was seen that made it necessary: after `flows`, `shape` and `parity` the agent knew what the module does and where it differs, but each session still invented its own list of what to test.

How it was checked:
- Two tests written first and failing (`unknown action "cases"`): the table with a case of each kind, the ones that must not appear (a field that is not required, the `show` endpoint as an empty list), the header and the legend; and the module without backend. After seeing the real output, one more fix: the `simulate` match had `{}` in it and would never match a real request; the test now pins `quotes/approve/`.
- `go test -race ./...` passes.
- Not checked: on the real quotes module (a long list, and whether 120 rows is too many or too few); the cases of a screen whose actions are not `@click` on a button (they are not in `flows` either); that the `run with` phrases are enough for an agent that has not read the skill.

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
