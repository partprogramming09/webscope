package code

import (
	_ "embed"
	"fmt"

	"github.com/partprogramming09/webscope/internal/input"
)

//go:embed playbook-migration.md
var migrationPlaybook string

// codeActions are the actions of code, in the order help lists them.
var codeActions = []string{"fields", "consumers", "map", "map_back", "flows", "cases", "legacy", "rows", "snap", "snap_diff", "leftovers", "help"}

const helpText = `code(action, args?) reads the project and the database; it never writes.
fields {table, tenant?}: columns of a table, rows with a value, who reads and writes each.
consumers {table, column} or {column_only}: each read and write of a column, file:line.
map {module, walkthrough?}: routes, calls and screens of a front module.
map_back {query, offset?}: from a screen or API address to endpoint, controller, service and table; when endpoints do not fit, the last line gives the offset of the next page.
flows {module}: screens, endpoints and tables together, as evidence for user stories.
cases {module}: the test cases of a module as a list; it runs nothing.
legacy {path, module?}: endpoints, callers and tables of a legacy PHP folder, against the new module's.
rows {table, a, b, key?, children?, tenant?}: how two rows differ, no values. a and b are {column: value} or an id (column key, id by default). children {table: column} counts the rows that point at each and, when the counts are equal, names the columns whose values differ.
snap {name, sql, key?, tenant?} then snap_diff {name}: what an action changed; key is a column, or several separated by commas.
leftovers {table, column or columns[≤10], prefix?, deleted? {column: value}, tenant?}: how many rows of a table have a column that starts with the test prefix (ZZ_ by default): what a test left behind; a count, no values.
help {topic?}: this list. Topics: migration.
Migrating a module, parity with a legacy system, user stories or test cases: code(help, {topic: "migration"}) gives the steps and the tool for each.`

// help answers the list of actions, or the playbook of a topic.
func help(args map[string]any) (string, error) {
	switch topic := input.Str(args, "topic"); topic {
	case "":
		return helpText, nil
	case "migration":
		return migrationPlaybook, nil
	default:
		return "", fmt.Errorf("no topic %q: the topics are migration. Next: code(help) lists the actions", echo(topic))
	}
}
