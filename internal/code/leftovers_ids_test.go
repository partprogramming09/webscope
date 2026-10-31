package code

import (
	"strings"
	"testing"
)

// TestLeftoversIdsListsTheLiveRowsAndAStatementToRunByHand: with ids the answer names the key of each live test row (not the soft-deleted ones) and gives
// the DELETE for the user to run; without live rows or without ids there is neither.
func TestLeftoversIdsListsTheLiveRowsAndAStatementToRunByHand(t *testing.T) {
	c := codeSetup(t)
	conn := writeDB(t, c)
	for _, q := range []string{
		"CREATE TABLE lo5 (id INTEGER, code TEXT, estado TEXT)",
		"INSERT INTO lo5 VALUES (1, 'ZZ_A', 'BORRADO'), (2, 'ZZ_B', 'ACTIVO'), (3, 'real', 'ACTIVO'), (4, 'ZZ_C', NULL)",
	} {
		if _, err := conn.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	args := map[string]any{"table": "lo5", "column": "code", "deleted": map[string]any{"estado": "BORRADO"}}
	if out, _ := code(t, c, "leftovers", args); strings.Contains(out, "DELETE") {
		t.Errorf("without ids there is no statement: %q", out)
	}
	args["ids"] = "id"
	out, err := code(t, c, "leftovers", args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "live ids (id): 2, 4") || !strings.Contains(out, "DELETE FROM lo5 WHERE id IN (2, 4);") || !strings.Contains(out, "webscope never writes") {
		t.Errorf("ids 2 and 4 are the live ones, 1 is soft-deleted and 3 is not a test row: %q", out)
	}
	if _, err := code(t, c, "leftovers", map[string]any{"table": "lo5", "column": "code", "ids": "id; x"}); err == nil {
		t.Error("a bad key name must be refused")
	}
	args["prefix"] = "NOPE_"
	if out, _ := code(t, c, "leftovers", args); strings.Contains(out, "DELETE") {
		t.Errorf("no live rows, no statement: %q", out)
	}
}
