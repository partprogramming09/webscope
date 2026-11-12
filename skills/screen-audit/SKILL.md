---
name: screen-audit
description: Review a screen of a web app for what is wrong with how it is drawn — text that was not rendered, fields that are missing or should not be there, controls covered by another element, fields or buttons with no name — and, when there is an old and a new version of the screen, compare them. Use with the webscope tools (page, code, query) when asked to audit, review or compare a screen, check a legacy screen against its replacement, or find UX defects in a module.
---

# Screen audit

`page audit` gives facts the page shows about itself. It cannot say what *should* be on the screen: that needs a reference, and the judgment is yours. Report findings as findings, never as certainties.

## Rules

- Read only. Do not save, delete or submit anything unless the user approved it; test rows are prefixed `ZZ_`. If a screen asks for a login, stop and ask the user for the session.
- One agent drives Chrome at a time. Run `page(load)` again after every navigation or reload; a legacy screen loses the helper on each link.
- `WEBSCOPE_APP_URL` must hold the origin of the tab; with an old and a new app it can hold both, separated by commas. With an origin that is in none of them `state` and `audit` still work and warn, but `payloads` shows nothing.
- Customer data: counts and schema, never rows.

## One screen

1. `page(load)`, run the snippet, then `page(state)`: headings, fields, buttons, alerts, table rows.
2. `page(audit)`, and `page(audit {selector})` on a dialog or a section when the whole screen is too noisy.
3. Read each category:
   - `unrendered`: `{{ x }}`, `undefined`, `null`, `NaN`, `[object Object]`, `$t(`. A real defect unless it is quoted text. Name the element and the text.
   - `covered`: the element at the control's centre is not the control. A wrapper of the control itself or a modal over the page is skipped by the audit; look at the rest with a screenshot before calling it a defect.
   - `no label`: a person or `fill` cannot name the field. The line gives `name`, `id` and a short CSS path.
   - `same label`: two fields answer to the same name (a filter and a form field, say). `fill` takes the first; say which one is meant.
   - `no name`: icon-only buttons and links. Common and often intended; list them as an accessibility gap, not as an error.
4. A finding is not a defect until you looked: take a screenshot (`computer`) of anything you will report as covered, overlapping or missing.

## Fields that are missing or should not be there

There is no reference inside the page, so bring one:

- The columns of the table behind the screen: `code(fields {table})` (counts, never values) and `code(map_back {query})` for its endpoint, service and table. A column the form writes that no field names, or a field with no column, is a candidate.
- The other version of the screen, when there is one: open each, run `page(shape, {name: "legacy"})` and `page(shape, {name: "modern"})` (when one side puts the list and the form on one page and the other splits them, give each a `selector` for the part to compare, such as the list or the dialog) (run `page(load)` again after each navigation, and put the same filters and the same dialog open on both sides), then `page(parity, {a: "legacy", b: "modern"})`: it prints only what one side has and the other lacks. If the screens are still not the same, `state` each and compare the lists by label. Different wording for the same thing is a note, not a defect. A field present on one side only is a finding.
- What was sent: after a save, `page(payloads)` and `check` show which fields travelled. To compare the old and the new screen, do the same action on both and run `page(parity, {req_a: "POST save.php", req_b: "POST /api/quotes", map: {OLD_NAME: "new_name"}})`: only the field names that differ come back. The old pages name fields in their own way, so build the `map` from `code fields` and the field list of the screen; names left unmapped show as a difference on both sides.

## Old against new

- Same module on both sides, same test data. Do one screen at a time; do not mix recordings of the two.
- Compare: headings, fields (label, type, required mark, options), buttons, tabs, table columns, alerts and their texts.
- Mark an intended hardening of the new side (a required field, a uniqueness check) as intended, not as a difference.
- The same raw value can mean different things on each side (an id that points at another catalog): confirm against the catalog before reporting it.

## Report

For each finding: what, where (label or CSS path), how you know (audit line, `state` line, screenshot), and how sure you are. End with what you did not check.
