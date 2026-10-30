Migration parity: what the new module lacks, adds or changes against its legacy, with evidence. Report findings; a person decides what is a defect. Read only: a write goes through the screen, on rows prefixed ZZ_, with the user's approval. Never print row values.

1. Stories. `code flows {module}`: screens, endpoints and tables. Write the user stories and have a person confirm them; without them there is nothing to compare against.
2. Legacy side. `code legacy {path, module}`: endpoints, their callers, the tables they write (W) or read (R), the tables only one side has, and the columns the legacy writes that the backend never names (three groups). A table only the legacy has and the backend names elsewhere is not a gap; one no file names is a question for the user.
3. Screens. Open the old and the new screen in two tabs (WEBSCOPE_APP_URL may list both origins). `page shape {name, selector?}` on each, then `page parity {a, b}`: fields, buttons and columns that only one side has. `page audit` on both. Run `page load` again after every navigation.
4. Requests. Do the same action on both, then `page parity {req_a, req_b, map}`: names only. A field only one side sends: ask whether the other fills it on the server.
5. Writes. Create a ZZ_ row on each system: `code snap` before, `code snap_diff` after, then `code rows {table, a, b, children}`: how the two rows differ and the child rows each has. `check` compares what the screen sent with what was stored.
6. Cases. `code cases {module}`: what to test (happy path, permission, required field, status, HTTP error, empty list). Run the chosen ones with `page`, and add one case per difference found.
7. Unit tests. Write each confirmed difference as: Legacy: input gives output. New: input gives output. Pin it in the lowest layer that owns the rule (service, request, policy), with no database and no row values, and hand the list to a tester. Never change an expected value to make a test pass: ask which side is right.
8. Report: finding, side, the command that showed it, how sure, and the case that covers it. End with what was not checked.

Limits: `page parity` compares names, not behaviour; `code flows`, `code cases` and `code legacy` read text, so a rule written in a service or in the old source files is found only by reading them; a shape is a photo of one moment.
