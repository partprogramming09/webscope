---
name: migration-parity
description: Check that a whole module migrated from a legacy system does what the legacy did: flows, screens, requests, saved data and test cases, chaining module-flows, screen-audit and parity-tests. Use when asked for parity between the old and the new version of a module, to verify a migration, or to list what to test before retiring a legacy screen. For one screen only, use screen-audit.
---

# Migration parity

Goal: a short, evidence-backed list of what the new module lacks, adds or changes against the legacy one, and the cases that prove the rest. Report findings as findings; a person decides what is a defect.

## Rules

Follow `../common-rules.md`. Open each side with the same filters and the same dialog.

## Steps

1. **Flows.** Load `module-flows` and run `code flows {module}` for the new module. Write the stories and ask the user to confirm them. Without confirmed stories the rest has nothing to compare against.
2. **The legacy side.** If `WEBSCOPE_LEGACY` is set (the `path` is relative to it; if you do not know the module's folder, list `WEBSCOPE_LEGACY` and search it for the name of the module's table), `code legacy {path, module}` gives the legacy endpoints, who calls them and the tables they write, and the tables that only the legacy or only the new module touches. A table only the legacy has is split in two: the ones the backend names elsewhere (another service, model or query: not a gap, but check that the flow reaches them) and the ones no backend file names, which are the candidates for a gap: read the legacy file that uses them and ask the user. Then open the legacy screen of each story. `page(state)` and `page(payloads)` after each action: what it shows, what it sends. If the legacy code is readable, `Read` it for the rules `page` cannot see (validations, calculated fields, who may do what).
3. **Screens.** Load `screen-audit` and follow its "Fields that are missing or should not be there" and "Old against new" parts on both sides: `shape`, `parity`, `audit`. A name in another language is a pair, not two findings.
4. **Requests.** Do the same action on both sides and run `page(parity, {req_a, req_b, map})`. Build `map` from `code fields` and the field lists of both screens. A field only one side sends is a finding: ask whether the other side fills it in on the server.
5. **Data.** For a write done by the screen on a `ZZ_` row, `check` the saved row against what was sent. Use `code consumers {table, column}` to see who reads a column that only one side writes.
6. **Cases.** `code cases {module}` gives a list: happy path per action, permission, required field empty, status, HTTP error and empty list. Run the ones the user asks for with `page`, and verify with `check` or `query` as each row says. Add a case for every difference found in steps 3 to 5: it is the regression test of that finding.
7. **Unit tests.** For the confirmed findings, load `parity-tests`: it turns each one into a plan of unit tests.
8. **Report.** One table: finding, side, how you know (the command), how sure you are, and the case that covers it. End with what was not checked.

## What each tool cannot say

- `parity` compares names, never behaviour; two screens with the same fields can calculate differently.
- `flows` and `cases` are deduced from text; a rule written in a service or in the legacy code is only found by reading it.
- A shape is a photo of one moment: a field drawn after a delay is missed.
