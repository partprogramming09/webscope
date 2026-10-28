# Changelog

One entry per version, newest first. Tool names are `query`, `page`, `check` and `code`. A change in what a tool prints is listed as a change, because a client may read it.

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
