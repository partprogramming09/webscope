# Rules shared by the webscope skills

- Read only. Do not save, delete or submit anything unless the user approved it; test rows are prefixed `ZZ_`. If a screen asks for a login, stop and ask the user for the session.
- Customer data: counts and schema, never row values. Do not name a customer. `code fields` and `check` give counts and differences.
- One agent drives Chrome at a time. Run `page(load)` again after every navigation or reload; a legacy screen loses the helper on each link.
- `WEBSCOPE_APP_URL` must hold the origin of the tab; with an old and a new app it can hold both, separated by commas. With an origin that is in none of them `state` and `audit` still work and warn, but `payloads` shows nothing.
- Report findings as findings, never as certainties: say how you know (command, screenshot) and how sure you are, and end with what you did not check. A person decides what is a defect.
