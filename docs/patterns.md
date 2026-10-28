# Known patterns

Findings met while testing screens with webscope 0.5 (`query`, `page`, `check`, `code`), each with how to recognize it and what to do. Check this list before testing a module; add a row when a new kind of finding appears. The last column says how far each one was verified: **measured** (seen in the Acme demo tenant, 24 to 28 October 2026), **test** (only in the Chrome test fixture) or **read** (from code or from an old session, not reproduced since).

Tools of the old server (`e2e_chrome`, `__f.*`, `payload_check`, …) are in [old-to-new-tools.md](old-to-new-tools.md).

## What the screen does

| Pattern | How to recognize it | What to do | Seen |
|---|---|---|---|
| Save disabled until valid | `state` lists `Guardar (disabled)` (from 0.5.16) and `click` on it answers `No effect seen` | Report «disabled», not «no errors»; fill one field at a time to find the rule that enables it (`charge-types`: Tipo, Tipo de Cargo, Base and Operación) | measured |
| Message only on screen | `click` or `press` answers `New alerts:` or `New errors:` and no `Requests:` | Client validation: record the text per field; never count it as a save | test |
| HTTP error with no notice, old data left | `simulate {match, status:500}` then an action that reloads the list: `Requests: GET … 500`, no `New alerts`, same rows as before | Report it (the user sees stale data and thinks it is current); S4 in [module-testing.md](module-testing.md) | measured twice (quotes list) |
| A screen calls an endpoint that needs a session before the login | `Requests: GET … 403` on a screen shown without a session (the reset-password screen called the settings endpoint) | Report it; it is noise in the log and a wrong request, not a data leak | measured in a session, not reproduced since |
| An action offered that the server always refuses | `Requests: PUT … 422` and a notice such as «cannot edit an approved quote» after a click on «Edit» | Report as a UX defect: hide or disable the action for that state | measured in a session |
| Handler that breaks without a visible effect | `state` shows `Console (…): warn: [Vue warn]: Unhandled error …` or `click` answers `New console errors:` | Read the line (component and handler named), then the code | test; the Acme case did not reproduce on 28 Oct |
| Actions in a row menu or icon-only buttons | The action is not in `state` (no text) or the menu is closed | `click` by `selector`; open the menu first and read it with `state` | read |
| Rate limit | `Requests: … 429` or a notice «Too Many Attempts» | Wait and repeat; never count it as a failure of the module | read |
| Session lost | `state` shows the login screen or `Requests: … 401` | Stop and ask the user for the session; never type credentials | read |

## `fill` and menus

| Pattern | How to recognize it | What to do | Seen |
|---|---|---|---|
| A menu keeps its old value | `fill` answers `the page shows "A" instead of "B"` | The field is a Vuetify menu: it reads the value back, so this is real; `fill` chooses in `v-select` and `v-autocomplete` by the text of the option | measured |
| Option with several parts | The field shows only the first line (`Demo`) and the option reads `Demo Documento: +57…` | Fixed in 0.5.8: the readback joined the selection and its inner text (`Demo, Demo`) and rejected a correct choice | measured |
| Autocomplete finds nothing for the whole text | `no option "…" among 0 seen` although the option exists | Fixed in 0.5.9: it retries with the first word; if it still fails the option is really missing | measured |
| Select empty because its source has no rows | `no option "X" among N seen: …` lists what the menu holds | Count the source with `query` (rows of that type or status) before blaming the form | read |
| Option text differs from the value you typed | `no option "Sí"` and the list shows `Si, No`; `Inactivo` against `Activas, Desactivadas` | Use the text the menu shows (the error lists it); a different label for the same thing is an S5 finding | measured in sessions |

## What is sent, stored and read

| Pattern | How to recognize it | What to do | Seen |
|---|---|---|---|
| Save without changes modifies data | `code snap` before, open the record and press «Edit» then save without touching anything, `code snap_diff` after: columns change (`'' → NULL`, a code replaced by a name) | The edit form sends a different representation than the create form (`MIA` against `Miami`); report it, it corrupts data on every untouched save | measured |
| Untouched save is clean | `snap` before, open the record, save without changes, `snap_diff`: «no changes» (`charge-types`, a `PUT` whose `check` gives all fields equal) | Not a defect; it shows the contrast with the module where it does modify data | measured |
| Same field, three representations | `check` shows `office  MIA  MIA  Miami` | A real difference when sent and stored agree and the read differs; a field the list names differently shows as `unread~`, not as an error | measured |
| `check` says `unread~` | The read does not name the field (the list calls it `description`, the POST `notes`) | Not a defect: pass `after` with the request that names it, or `at` to point at the row | measured |
| Delete is logical | After deleting by screen the row stays with `ESTADO = BORRADO` and its child rows stay too | Count by `ESTADO`, not the total; `query` cannot clean it (it never writes): clean through the screen and list the ids left | measured |
| Leading zeros lost | `check` would show `007  007  7` (sent, stored, read) | A type problem between the form, the rule and the column: S1; `007` travelled intact in the real cycles so far | test |
| `Unknown column` or `Unknown table` | `query` answers `Error 1054` or `Error 1146` | The answer says how to see the real ones: `code fields {table}`, `SHOW COLUMNS FROM table`, `SHOW TABLES LIKE '%word%'`; do not guess names. With a `tenant`, a missing table may live in the central database: call without `tenant` (said by the answer from 0.5.18) | measured (about 8 % of the calls before 0.5.11; two missing tables in one session were central or misnamed) |

## Running it

| Pattern | How to recognize it | What to do | Seen |
|---|---|---|---|
| `payloads` shows 0 requests | `state` works but nothing is captured; since 0.5.5 `payloads` says a request from another origin was refused | `WEBSCOPE_APP_URL` must equal the tab's exact origin (`demo.localhost:5173`, not `localhost:5173`); change it, run `/mcp` and `page(load)` again | measured |
| `page` has no tab with a session | `Failed: webscope is not loaded in this page` or the login shows | Open a tab of the app in the MCP group with `tabs_create_mcp` and navigate; if it shows the login, stop and ask | measured |
| Console lines missing | `state` shows no `Console` line although the page failed earlier | The helper logs only from `page(load)` on: load it first, then act | test |
| `javascript_tool` answers `[BLOCKED: Cookie/query string data]` | The output carries an address with a query string | `Requests:` already drops the query; for other text return only the lines you need, filtered | measured |
| `find` and clicks fail on an empty tree | The accessibility tree holds only `ref_1` right after a navigation | Wait for the screen to draw, then read `state`; this is Claude in Chrome, not `page` | measured |
| Two drivers on one Chrome | Tabs jump to other routes, the session drops | Drive the browser from one place at a time | read |
| A downloaded binary dies with exit 137 | A `webscope` binary with the `com.apple.quarantine` attribute (macOS) | `xattr -d com.apple.quarantine <file>` | read |
