---
name: module-flows
description: Find the flows and user stories of a module or project from its code, with evidence, and have a person confirm them. Use with the webscope tools (code, page, query) when asked what a module does, for its user stories or flows, before migrating a module from a legacy system, or before writing test cases for it.
---

# Module flows

`code flows {module}` gives evidence: the screens of the module with their actions, and the endpoints, controllers and tables behind them. It does not give stories. The code can describe a mistake as a feature, so a person confirms the stories.

## Steps

1. `code flows {module}` (a part of the front path, such as `crm/quotes`). If it says `WEBSCOPE_BACKEND is not set`, you have only the screens: say so.
2. One story per action that changes something or leads to another screen. Write `As <who> I want <action> so that <result>`. The `who` comes from the permission (`can('quotes.create')`); if there is none, write `any user` and mark it as a guess.
3. Under each story write its evidence: the action, the endpoint (`METHOD uri`), the controller method with `file:line` and the table. A story without evidence is a question for the user, not a story.
4. Mark what the code cannot tell you: business rules that live in a service (`Read` it), statuses and their transitions (`statuses` line of the answer), and endpoints with `api without a route in the backend`.
5. Check one flow in the real screen: `page state`, then `page payloads` after the action, and `check` against the row. If it differs from the story, the story is wrong or the code has a bug: report both.
6. Give the list to the user and ask them to confirm, correct or delete each story before anyone writes tests from it.

## Rules

- Read only. Never save, delete or submit without the user's approval; test rows are prefixed `ZZ_`.
- Every line the answer gives is an estimate by text, not proof. Say which stories you confirmed in the screen and which not.
- Do not name a customer or show row values; `fields` gives counts only.
