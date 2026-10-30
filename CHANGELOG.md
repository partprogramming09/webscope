# Changelog

One entry per version, newest first. Tool names are `query`, `page`, `check` and `code`. Each change says what it is, what was seen that made it necessary, how it was checked and what was not checked. A change in what a tool prints is listed as a change, because a client may read it.

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
