package main

import (
	"strings"
	"testing"
)

func TestCheckSQLAccepts(t *testing.T) {
	for _, q := range []string{
		"SELECT 1", "select * from t;", "  SELECT 1 ;; ", "WITH a AS (SELECT 1) SELECT * FROM a",
		"SHOW TABLES", "DESCRIBE t", "DESC t", "EXPLAIN SELECT 1",
		"SELECT ';' AS x",                                  // a ; inside a string is not a second statement
		"SELECT 'it''s; fine'",                             // doubled quote inside a string
		"SELECT 'into' AS w",                               // INTO inside a string is only text
		"SELECT `into` FROM t",                             // and so is a quoted identifier
		"SELECT 1 -- ; DROP TABLE t",                       // a comment runs to the end of the line
		"SELECT 1 # ; DROP TABLE t",                        // MySQL's # comment too
		"SELECT 1 /* ; DELETE */",                          // a block comment
		"-- note\nSELECT 1",                                // a comment before the statement
		"SELECT updated_at FROM t WHERE action = 'delete'", // words that merely contain or sit next to a verb
	} {
		if _, err := checkSQL(q); err != nil {
			t.Errorf("%q was refused: %v", q, err)
		}
	}
}

func TestCheckSQLRefuses(t *testing.T) {
	for q, want := range map[string]string{
		"UPDATE t SET a = 1":                    "must start with",
		"DELETE FROM t":                         "must start with",
		"INSERT INTO t VALUES (1)":              "must start with",
		"DROP TABLE t":                          "must start with",
		"CREATE TABLE x (a int)":                "must start with",
		"TRUNCATE t":                            "must start with",
		"ALTER TABLE t ADD c int":               "must start with",
		"CALL proc()":                           "must start with",
		"SET @a = 1":                            "must start with",
		"-- hi\nDELETE FROM t":                  "must start with",
		"":                                      "empty",
		"  ;  ":                                 "empty",
		"SELECT 1; DROP TABLE t":                "one statement",
		"SELECT 1;SELECT 2":                     "one statement",
		"SELECT * INTO OUTFILE '/tmp/x' FROM t": "INTO",
		"SELECT 1 INTO @v":                      "INTO",
		"/*!50000 DROP TABLE t */ SELECT 1":     "executable comment",
		"SELECT 1 /*! ; DROP TABLE t */":        "executable comment",
		"SELECT 'unterminated":                  "unterminated",
		"SELECT 1 /* open":                      "unterminated",
	} {
		_, err := checkSQL(q)
		if err == nil {
			t.Errorf("%q was accepted", q)
		} else if !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %q does not mention %q", q, err, want)
		}
	}
}

func TestCheckSQLReturnsTheStatementWithoutTrailingSemicolons(t *testing.T) {
	got, err := checkSQL("  SELECT 1 ;; ")
	if err != nil || got != "SELECT 1" {
		t.Fatalf("got %q, %v; want %q", got, err, "SELECT 1")
	}
}

func TestTheGuardReadsAStatementUnderTheRulesOfBothEngines(t *testing.T) {
	for _, q := range []string{
		"SELECT 1 AS [a']; COMMIT; PRAGMA query_only=0; DELETE FROM t; SELECT 1 AS [b']", // SQLite: a quote inside [identifier]
		"SELECT 'a\\'; DELETE FROM t; SELECT 'b\\'",                                      // SQLite: no backslash escape, so these are three statements
		"SELECT [unterminated",
	} {
		if out, err := checkSQL(q); err == nil {
			t.Errorf("%q was let through as %q", q, out)
		}
	}
	for _, q := range []string{"SELECT '[x]' AS a, \"[y]\" AS b", "SELECT 'it''s' ; ", "SELECT json_extract(j, '$[0]') FROM t"} {
		if _, err := checkSQL(q); err != nil {
			t.Errorf("%q is one read and was refused: %v", q, err)
		}
	}
}

func TestIntoStuckToAnAtSignIsStillInto(t *testing.T) {
	for _, q := range []string{"SELECT 1 INTO@x", "select 1 into@@session.x", "SELECT 1 FROM t INTO OUTFILE'/tmp/x'", "SELECT 1 INTO/**/DUMPFILE'/tmp/x'"} {
		if out, err := checkSQL(q); err == nil {
			t.Errorf("%q was let through as %q", q, out)
		}
	}
	if _, err := checkSQL("SELECT @a, @@version, 'INTO' AS w"); err != nil {
		t.Errorf("variables and the word in a string are reads: %v", err)
	}
}
