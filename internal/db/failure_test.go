package db

import (
	"errors"
	"strings"
	"testing"
)

func TestFailureSaysHowToSeeTheRealColumnsAndTables(t *testing.T) {
	for _, tc := range []struct{ err, want string }{
		{"Error 1054 (42S22): Unknown column 'pantalla' in 'field list'", "code(fields"},
		{"SQL logic error: no such column: pantalla (1)", "code(fields"},
		{"Error 1146 (42S02): Table 'db.nada' doesn't exist", "SHOW TABLES"},
		{"SQL logic error: no such table: nada (1)", "SHOW TABLES"},
		{"Error 1064 (42000): You have an error in your SQL syntax", "fix the statement"},
	} {
		got := Failure(errors.New(tc.err), "")
		if !strings.HasPrefix(got, "Failed: "+tc.err) || !strings.Contains(got, "Next: ") || !strings.Contains(got, tc.want) {
			t.Errorf("%s: %q lacks %q", tc.err, got, tc.want)
		}
	}
}

func TestFailureOfAMissingTableWithATenantSaysACentralTableIsReadWithoutIt(t *testing.T) {
	for _, e := range []string{"Error 1146 (42S02): Table 'db.customers' doesn't exist", "SQL logic error: no such table: customers (1)"} {
		with := Failure(errors.New(e), "demo.localhost")
		if !strings.Contains(with, "without tenant") || !strings.Contains(with, "SHOW TABLES") {
			t.Errorf("%s with a tenant: %q", e, with)
		}
		if without := Failure(errors.New(e), ""); strings.Contains(without, "tenant") {
			t.Errorf("%s without a tenant must not mention it: %q", e, without)
		}
	}
	if got := Failure(errors.New("Error 1054 (42S22): Unknown column 'x' in 'field list'"), "demo.localhost"); strings.Contains(got, "without tenant") {
		t.Errorf("a missing column is not a central table: %q", got)
	}
}

func TestFailureTextHasNoDoubledPeriodWhenTheErrorEndsInOne(t *testing.T) {
	got := Failure(errors.New("Error 1792 (25006): Cannot execute statement in a READ ONLY transaction."), "")
	if strings.Contains(got, "..") {
		t.Fatalf("doubled period in %q", got)
	}
	if !strings.HasPrefix(got, "Failed: Error 1792") || !strings.Contains(got, "transaction. Next: ") {
		t.Fatalf("unexpected text %q", got)
	}
}
