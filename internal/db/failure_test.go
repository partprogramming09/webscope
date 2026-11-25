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

func TestFailureOfAWriteSaysTheServerNeverWritesAndToUseTheScreen(t *testing.T) {
	for _, e := range []string{
		`the statement must start with SELECT, SHOW, DESCRIBE, DESC, EXPLAIN or WITH, not "UPDATE"`,
		`the statement must start with SELECT, SHOW, DESCRIBE, DESC, EXPLAIN or WITH, not "insert"`,
		`a WITH that ends in DELETE is a write`,
	} {
		got := Failure(errors.New(e), "")
		if !strings.Contains(got, "never writes") || !strings.Contains(got, "screen") || strings.Contains(got, "WEBSCOPE_DB_URL") {
			t.Errorf("%s: %q", e, got)
		}
	}
	if got := Failure(errors.New(`the statement must start with SELECT, SHOW, DESCRIBE, DESC, EXPLAIN or WITH, not "SELEC"`), ""); strings.Contains(got, "never writes") {
		t.Errorf("a typo is not a write: %q", got)
	}
}

func TestFailureNamesTheConnectionProblemAndPointsToDoctor(t *testing.T) {
	for _, tc := range []struct{ err, want string }{
		{"WEBSCOPE_DB_URL is not set", "set WEBSCOPE_DB_URL"},
		{"WEBSCOPE_DB_URL is not a valid URL", "a valid URL"},
		{"dial tcp 127.0.0.1:3306: connect: connection refused", "not reachable"},
		{"Error 1045 (28000): Access denied for user 'ro'@'172.19.0.1' (using password: YES)", "read-only user"},
	} {
		got := Failure(errors.New(tc.err), "")
		if !strings.Contains(got, tc.want) || !strings.Contains(got, "code(doctor)") || strings.Contains(got, "fix the statement") {
			t.Errorf("%s: %q lacks %q or code(doctor), or blames the statement", tc.err, got, tc.want)
		}
	}
}

func TestFailureOfADatabaseTheUserMayNotOpenIsNotAWrongPasswordAndASlowReadIsNotAnUnreachableHost(t *testing.T) {
	got := Failure(errors.New("Error 1044 (42000): Access denied for user 'ro'@'%' to database 'tenant_x'"), "")
	if strings.Contains(got, "password") {
		t.Errorf("a refused database is not a wrong password: %q", got)
	}
	if got := Failure(errors.New("read tcp 127.0.0.1:5000->127.0.0.1:3306: i/o timeout"), ""); strings.Contains(got, "not reachable") {
		t.Errorf("a read that timed out is not an unreachable host: %q", got)
	}
	if got := Failure(errors.New("dial tcp 10.0.0.9:3306: i/o timeout"), ""); !strings.Contains(got, "not reachable") {
		t.Errorf("a dial that timed out is: %q", got)
	}
}
