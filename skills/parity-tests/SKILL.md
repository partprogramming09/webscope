---
name: parity-tests
description: Turn what a legacy-to-new comparison found into a plan for unit tests of the new code, so the new module is held to what the legacy did. Use after migration-parity, module-flows or code cases, when asked to write or plan unit tests from the legacy behaviour, to contrast functionality between the old and the new version, or to hand a tester what to test.
---

# Parity tests

The comparison tools say what differs; they do not say who is right. A unit test says it. Write tests only from facts a person confirmed, and pin the behaviour the legacy had.

## Inputs

- Confirmed stories from `module-flows` and the report of `migration-parity` (findings, each with the command that showed it).
- `code cases {module}` for the case kinds; `code map_back` for the controller, service and model each case reaches.
- The legacy behaviour: what `page(state)` and `page(payloads)` showed on the old screen, and the legacy code if it can be read.

## Steps

1. **One rule per finding.** Write each difference as `Legacy: <input> gives <output>. New: <input> gives <output>.` Take both sides from evidence, never from memory. Mark any rule you only deduced from a name or a text search as `unconfirmed` and ask the user before it becomes a test.
2. **Pick the lowest layer that owns the rule.** A calculation or a status change belongs to a service: test the service. A required field or a format belongs to the request or validator class. An allowed or refused action belongs to a permission or policy test. Only what no lower layer can show (a button that is not drawn) stays a screen case for `page`. Use `code map_back` to find the class; read it before deciding.
3. **One case per behaviour.** From each rule take the happy case, the edge that the legacy handled (empty, zero, maximum length, a repeated value, a leading zero such as `007`) and the refusal. Name each test with the rule it pins, in the words of the story.
4. **Say what the test must do today.** For each test write whether it should pass now or fail now. A failing one is the point of the exercise: it is a confirmed regression. Never edit the expected value of a test to make it pass; ask the user whether the legacy or the new behaviour is the right one.
5. **Follow the project.** If you have a skill for the project's stack, load it. Read two existing tests before writing: same framework, same folder, same factories and naming. If the project has no tests, say so and ask before creating a structure.
6. **Hand over.** Give whoever writes the tests (a tester agent if you have one) the list: layer, class, the rule, the cases with input and expected output, and pass or fail today. They write the tests; you do not write them in the same pass you planned them.
7. **Prove them.** Run the new tests. Each one that should fail must fail for the reason written down; then plant one regression in the code (a wrong status, a dropped validation), see the matching test fail, and undo it. A test that never failed is not proof.

## Rules

- No row values in tests or in the plan: build data with factories, or with rows prefixed `ZZ_` through the screen.
- A test does not call the real legacy system. The legacy side of a test is a fixed expected value taken from evidence, written next to where it came from.
- Do not test names: two screens with the same fields can calculate differently, and `parity` only compares names.
- End the plan with what is not covered and why.
