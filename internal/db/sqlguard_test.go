package db

import (
	"database/sql"
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
		if _, err := CheckSQL(q); err != nil {
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
		"(SELECT 1) UNION ALL (SELECT 2)":       "SQLite rejects a parenthesized",
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
		_, err := CheckSQL(q)
		if err == nil {
			t.Errorf("%q was accepted", q)
		} else if !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %q does not mention %q", q, err, want)
		}
	}
}

func TestCheckSQLReturnsTheStatementWithoutTrailingSemicolons(t *testing.T) {
	got, err := CheckSQL("  SELECT 1 ;; ")
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
		if out, err := CheckSQL(q); err == nil {
			t.Errorf("%q was let through as %q", q, out)
		}
	}
	for _, q := range []string{"SELECT '[x]' AS a, \"[y]\" AS b", "SELECT 'it''s' ; ", "SELECT json_extract(j, '$[0]') FROM t"} {
		if _, err := CheckSQL(q); err != nil {
			t.Errorf("%q is one read and was refused: %v", q, err)
		}
	}
}

func TestIntoStuckToAnAtSignIsStillInto(t *testing.T) {
	for _, q := range []string{"SELECT 1 INTO@x", "select 1 into@@session.x", "SELECT 1 FROM t INTO OUTFILE'/tmp/x'", "SELECT 1 INTO/**/DUMPFILE'/tmp/x'"} {
		if out, err := CheckSQL(q); err == nil {
			t.Errorf("%q was let through as %q", q, out)
		}
	}
	if _, err := CheckSQL("SELECT @a, @@version, 'INTO' AS w"); err != nil {
		t.Errorf("variables and the word in a string are reads: %v", err)
	}
}

func TestADoubleDashIsACommentForSQLiteEvenWithoutASpaceAfterIt(t *testing.T) {
	for _, q := range []string{"SELECT 1 --'\n;DELETE FROM items;'", "SELECT 1 --\"\n;DELETE FROM items;\"", "SELECT 1 --`\n;DELETE FROM items;`", "SELECT 1 --[\n;DELETE FROM items;]"} {
		if out, err := CheckSQL(q); err == nil {
			t.Errorf("%q was let through as %q", q, out)
		}
	}
	if _, err := CheckSQL("SELECT 1 -- a comment\n, 2"); err != nil {
		t.Errorf("a comment with a space is a comment: %v", err)
	}
}

func TestPragmaIsRefusedWhereverItAppears(t *testing.T) {
	for _, q := range []string{"EXPLAIN PRAGMA query_only=0", "explain pragma writable_schema=1", "EXPLAIN PRAGMA journal_mode=wal"} {
		if _, err := CheckSQL(q); err == nil {
			t.Errorf("%q was let through", q)
		}
	}
	if _, err := CheckSQL("SELECT name FROM pragma_table_info('items')"); err != nil {
		t.Errorf("the table function pragma_table_info is a read: %v", err)
	}
}

func TestAFunctionNameBetweenQuotesIsStillTheFunction(t *testing.T) {
	for _, q := range []string{"SELECT `load_file`('/etc/hosts')", "SELECT `GET_LOCK`('a', 1)", "SELECT \"load_file\"('/etc/hosts')", "SELECT [pragma]"} {
		if _, err := CheckSQL(q); err == nil {
			t.Errorf("%q was let through", q)
		}
	}
	if _, err := CheckSQL("SELECT `name` FROM `items` WHERE `code` = 'x'"); err != nil {
		t.Errorf("ordinary quoted names are fine: %v", err)
	}
}

// Whatever the guard accepts must not write, even without the read-only open of SQLite: every combination of quote and comment characters
// around a hidden DELETE is run on a scratch in-memory database when CheckSQL lets it through.
func TestNoStatementTheGuardAcceptsWritesInSQLite(t *testing.T) {
	alphabet := []string{"'", "\"", "`", "[", "]", "-", "#", "/", "*", "\\", "\n", " ", "!"}
	var words []string
	var build func(prefix string, left int)
	build = func(prefix string, left int) {
		words = append(words, prefix)
		if left == 0 {
			return
		}
		for _, c := range alphabet {
			build(prefix+c, left-1)
		}
	}
	build("", 3)
	tails := append([]string{""}, alphabet...)
	for _, c := range alphabet {
		tails = append(tails, c+c, c+"\n")
	}
	db, err := sql.Open("sqlite", "file:fuzz?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE t (a)"); err != nil {
		t.Fatal(err)
	}
	accepted, tried := 0, 0
	for _, p := range words {
		for _, q := range tails {
			for _, stmt := range []string{"SELECT 1 " + p + ";\nDELETE FROM t;\n" + q, "SELECT 1 " + p + "\n;DELETE FROM t;" + q, "SELECT 1 " + p + "\nDELETE FROM t;" + q} {
				tried++
				if _, err := CheckSQL(stmt); err != nil {
					continue
				}
				accepted++
				if _, err := db.Exec("INSERT INTO t VALUES (1)"); err != nil {
					t.Fatal(err)
				}
				rows, err := db.Query(stmt)
				if err == nil {
					rows.Close()
				}
				var n int
				db.QueryRow("SELECT count(*) FROM t").Scan(&n)
				if n == 0 {
					t.Fatalf("CheckSQL accepted %q and it deleted the rows", stmt)
				}
				db.Exec("DELETE FROM t")
				db.Exec("CREATE TABLE IF NOT EXISTS t (a)")
			}
		}
	}
	t.Logf("%d statements tried, %d accepted by the guard, none wrote", tried, accepted)
}

func TestAWithThatEndsInAWriteIsRefusedByTheGuardItselfAndATableFunctionIsNot(t *testing.T) {
	for _, q := range []string{
		"WITH x AS (SELECT 1) DELETE FROM t", "with x as (select 1) update t set a = 99", "WITH x AS (SELECT 1) INSERT t SELECT * FROM x",
		"WITH x AS (SELECT 1) REPLACE t SELECT * FROM x", "EXPLAIN ATTACH DATABASE 'f.db' AS evil", "SELECT 1 FROM t FOR UPDATE", "SELECT 1; DETACH evil",
	} {
		if out, err := CheckSQL(q); err == nil {
			t.Errorf("%q was let through as %q", q, out)
		}
	}
	for _, q := range []string{
		"WITH x AS (SELECT 1 AS a) SELECT REPLACE(a, '1', '2'), INSERT('abc', 1, 1, 'x') FROM x", "WITH updates AS (SELECT 1 AS deleted) SELECT deleted FROM updates",
		"SELECT `update`, `delete` FROM t", "WITH x AS (SELECT 'DELETE UPDATE' AS w) SELECT w FROM x",
	} {
		if _, err := CheckSQL(q); err != nil {
			t.Errorf("%q is a read and was refused: %v", q, err)
		}
	}
}
