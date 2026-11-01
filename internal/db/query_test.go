package db

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/partprogramming09/webscope/internal/input"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture writes a SQLite file with 122 rows in items and returns a source that opens it read-only.
func fixture(t *testing.T) *Source {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		"CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, note TEXT, code TEXT)",
		"INSERT INTO items VALUES (1, 'ñandú 😀', NULL, '007')",
		"INSERT INTO items VALUES (2, 'tab' || char(9) || 'here', 'line1' || char(10) || 'line2', '0')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for i := 3; i <= 122; i++ {
		if _, err := db.Exec("INSERT INTO items VALUES (?, ?, ?, ?)", i, fmt.Sprintf("row %d", i), strings.Repeat("n", 1000), "x"); err != nil {
			t.Fatal(err)
		}
	}
	return &Source{URL: "sqlite://" + path}
}

func ask(t *testing.T, s *Source, q string, limit int) (string, error) {
	t.Helper()
	return s.Answer(context.Background(), QueryArgs{SQL: q, Limit: limit})
}

func TestQueryReturnsTSVWithHeaderNullAndNoControlCharacters(t *testing.T) {
	out, err := ask(t, fixture(t), "SELECT id, name, note, code FROM items WHERE id <= 2 ORDER BY id", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "id\tname\tnote\tcode\n1\tñandú 😀\t\\N\t007\n2\ttab here\tline1 line2\t0\n(2 rows)"
	if out != want {
		t.Fatalf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestQueryStopsAtTheRowLimit(t *testing.T) {
	out, err := ask(t, fixture(t), "SELECT id FROM items ORDER BY id", 50)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 52 || !strings.Contains(lines[51], "50 rows shown") || !strings.Contains(lines[51], "more exist") {
		t.Fatalf("%d lines, last %q", len(lines), lines[len(lines)-1])
	}
}

func TestRowLimitDefaultsAndIsCapped(t *testing.T) {
	for in, want := range map[int]int{0: 50, -3: 50, 7: 7, 500: 500, 99999: 500} {
		if got := rowLimit(in); got != want {
			t.Errorf("rowLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestQueryCutsLongCells(t *testing.T) {
	out, err := ask(t, fixture(t), "SELECT note FROM items WHERE id = 3", 0)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("no data line in %q", out)
	}
	if cell := lines[1]; !strings.HasSuffix(cell, "…") || len([]rune(cell)) != 201 {
		t.Fatalf("cell has %d runes: %q", len([]rune(cell)), cell)
	}
}

func TestQueryStopsAtTheByteCap(t *testing.T) {
	out, err := ask(t, fixture(t), "SELECT id, substr(replace(hex(randomblob(150)), '0', 'x'), 1, 190) AS pad FROM items", 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > input.MaxAnswer+200 || !strings.Contains(out, "cut at 8000 bytes") {
		t.Fatalf("%d bytes, last line %q", len(out), out[strings.LastIndex(out, "\n")+1:])
	}
}

func TestQueryRefusesAWriteAndTheDatabaseRefusesOneThatSlipsPastTheGuard(t *testing.T) {
	s := fixture(t)
	if _, err := ask(t, s, "UPDATE items SET name = 'x'", 0); err == nil {
		t.Fatal("UPDATE was not refused")
	}
	// A CTE that ends in DELETE starts with WITH, so the word check lets it through; the read-only session must stop it.
	if _, err := ask(t, s, "WITH d AS (SELECT 1) DELETE FROM items", 0); err == nil {
		t.Fatal("a DELETE behind WITH was not refused by the database")
	}
	out, err := ask(t, s, "SELECT count(*) AS n FROM items", 0)
	if err != nil || out != "n\n122\n(1 rows)" {
		t.Fatalf("the data changed or cannot be read: %q, %v", out, err)
	}
}

func TestQueryOnAMissingTableSaysSo(t *testing.T) {
	_, err := ask(t, fixture(t), "SELECT * FROM nope", 0)
	if err == nil || !strings.Contains(err.Error(), "no such table") {
		t.Fatalf("error = %v", err)
	}
}

func TestUnsetOrUnsupportedURLIsAClearError(t *testing.T) {
	if _, err := ask(t, &Source{}, "SELECT 1", 0); err == nil || !strings.Contains(err.Error(), "WEBSCOPE_DB_URL") {
		t.Errorf("unset: %v", err)
	}
	if _, err := ask(t, &Source{URL: "oracle://u:p@h/db"}, "SELECT 1", 0); err == nil || !strings.Contains(err.Error(), "mysql or sqlite") {
		t.Errorf("unsupported: %v", err)
	}
}

func TestThePasswordNeverAppearsInAnError(t *testing.T) {
	_, err := ask(t, &Source{URL: "mysql://user:s3cr3t-pw@127.0.0.1:1/db"}, "SELECT 1", 0)
	if err == nil {
		t.Fatal("a closed port gave no error")
	}
	if strings.Contains(err.Error(), "s3cr3t-pw") {
		t.Fatalf("the password leaked: %v", err)
	}
	if got := Redact("denied for s3cr3t-pw and again s3cr3t-pw", "s3cr3t-pw"); strings.Contains(got, "s3cr3t-pw") || got != "denied for *** and again ***" {
		t.Fatalf("redact = %q", got)
	}
}

// A statement the guard reads as one read can be several for SQLite: a quote inside [a'] (SQLite's bracket identifier) or a backslash before a quote
// (SQLite has no backslash escapes) hides what follows from the mask. The database must refuse the write whatever the mask decides.
func TestQueryRefusesStatementsThatHideAWriteFromTheMask(t *testing.T) {
	for name, q := range map[string]string{
		"bracket identifier":  "SELECT 1 AS [a']; COMMIT; PRAGMA query_only=0; DELETE FROM items; SELECT 1 AS [b']",
		"backslash":           "SELECT 'a\\'; COMMIT; PRAGMA query_only=0; DELETE FROM items; SELECT 'b\\'",
		"backslash no commit": "SELECT 'a\\'; DELETE FROM items; SELECT 'b\\'",
	} {
		s := fixture(t)
		if _, err := ask(t, s, q, 0); err == nil {
			t.Errorf("%s: the guard let %q through", name, q)
		}
		db, err := sql.Open("sqlite", strings.TrimPrefix(s.URL, "sqlite://"))
		if err != nil {
			t.Fatal(err)
		}
		var n int
		db.QueryRow("SELECT count(*) FROM items").Scan(&n)
		db.Close()
		if n != 122 {
			t.Errorf("%s: the table has %d rows, it had 122: a write got through", name, n)
		}
	}
}

func TestSQLiteIsOpenedReadOnlyAndAMissingFileIsNotCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.db")
	if _, err := ask(t, &Source{URL: "sqlite://" + path}, "SELECT 1", 0); err == nil {
		t.Error("a database that does not exist must be an error")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("opening a path that does not exist created the file")
	}
}

func TestQueryStopsAtMaxBytesEvenWithAFirstRowThatIsTooBig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wide.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cols, vals := make([]string, 100), make([]string, 100)
	for i := range cols {
		cols[i], vals[i] = fmt.Sprintf("c%03d TEXT", i), "'"+strings.Repeat("x", 300)+"'"
	}
	if _, err := db.Exec("CREATE TABLE w (" + strings.Join(cols, ",") + ")"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO w VALUES (" + strings.Join(vals, ",") + ")"); err != nil {
		t.Fatal(err)
	}
	out, err := ask(t, &Source{URL: "sqlite://" + path}, "SELECT * FROM w", 0)
	if err != nil || len(out) > input.MaxAnswer+200 || !strings.Contains(out, "cut at") {
		t.Errorf("%d bytes, %v", len(out), err)
	}
}

func TestTheGuardRefusesTheFunctionsThatReadServerFilesOrTakeNamedLocks(t *testing.T) {
	for _, q := range []string{"SELECT LOAD_FILE('/etc/hosts')", "select load_file(0x2f)", "SELECT GET_LOCK('a', 1)", "SELECT get_lock('a',1)"} {
		if _, err := CheckSQL(q); err == nil {
			t.Errorf("%q was let through", q)
		}
	}
	if _, err := CheckSQL("SELECT 'LOAD_FILE and GET_LOCK in a text' AS a"); err != nil {
		t.Errorf("the words inside a string are text: %v", err)
	}
}

func TestAHugeCellIsCutBeforeItIsConverted(t *testing.T) {
	if len(cell(strings.Repeat("ñ", 5_000_000))) > 4*maxCell {
		t.Error("a huge cell must be cut before it is converted")
	}
}

func TestTheHeaderOfAnAnswerKeepsEveryColumnName(t *testing.T) {
	out, err := ask(t, fixture(t), "SELECT id AS identifier_number_one, name AS the_name_of_the_item_that_is_long, code AS c FROM items WHERE id = 1", 0)
	if err != nil || !strings.HasPrefix(out, "identifier_number_one\tthe_name_of_the_item_that_is_long\tc\n") {
		t.Errorf("%q, %v", out, err)
	}
}

// withTables adds tables (name → "col type, ...") to the SQLite file behind s.
func withTables(t *testing.T, s *Source, tables map[string]string) {
	t.Helper()
	db, err := sql.Open("sqlite", strings.TrimPrefix(s.URL, "sqlite://"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for name, cols := range tables {
		if _, err := db.Exec("CREATE TABLE `" + name + "` (" + cols + ")"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnUnknownColumnOrTableNamesTheRealColumnsOfTheTablesInTheStatement(t *testing.T) {
	var wide []string
	for i := 1; i <= 60; i++ {
		wide = append(wide, fmt.Sprintf("c%d INTEGER", i))
	}
	s := fixture(t)
	withTables(t, s, map[string]string{"tags": "item INTEGER, label TEXT", "ñandú": "año TEXT", "wide": strings.Join(wide, ", ")})
	for _, tc := range []struct {
		name, sql string
		want      []string
		not       []string
	}{
		{"the ones that resemble the guess come first", "SELECT name_x FROM items", []string{"items(name, id, note, code)"}, nil},
		{"alias.col, a JOIN and every table listed", "SELECT a.zz FROM items a JOIN tags b ON b.item = a.id", []string{"items(", "tags(item, label)"}, nil},
		{"a missing table is skipped, the others are listed", "SELECT 1 FROM items JOIN nada ON 1 = 1", []string{"no such table: nada", "items("}, []string{"nada("}},
		{"non-ASCII names and quoted identifiers", "SELECT x FROM `ñandú`", []string{"ñandú(año)"}, nil},
		{"a wide table is cut", "SELECT zz FROM wide", []string{"wide(", ", …)"}, []string{"c41"}},
		{"only a missing table gives no hint", "SELECT 1 FROM nada", []string{"no such table: nada"}, []string{"Columns of"}},
		{"another error gives no hint", "SELECT FROM items", []string{"syntax error"}, []string{"Columns of"}},
	} {
		_, err := ask(t, s, tc.sql, 0)
		if err == nil {
			t.Errorf("%s: no error", tc.name)
			continue
		}
		for _, w := range tc.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: %q lacks %q", tc.name, err, w)
			}
		}
		for _, n := range tc.not {
			if strings.Contains(err.Error(), n) {
				t.Errorf("%s: %q has %q", tc.name, err, n)
			}
		}
	}
	_, err := ask(t, s, "SELECT zz FROM tags", 0)
	if text := Failure(err, ""); !strings.HasPrefix(text, "Failed: ") || !strings.Contains(text, "tags(item, label). Next: ") {
		t.Errorf("as the tool says it: %q", text)
	}
}

func TestAnUnknownColumnSuggestsTheNearestOneOnlyWhenOneIsClose(t *testing.T) {
	s := fixture(t)
	for _, tc := range []struct {
		sql  string
		want string
		not  string
	}{
		{"SELECT nmae FROM items", "Did you mean «name»?", ""},
		{"SELECT codee FROM items", "Did you mean «code»?", ""},
		{"SELECT name_x FROM items", "Did you mean «name»?", ""},
		{"SELECT zzzzzz FROM items", "Columns of the tables", "Did you mean"},
	} {
		_, err := ask(t, s, tc.sql, 0)
		if err == nil || !strings.Contains(err.Error(), tc.want) || (tc.not != "" && strings.Contains(err.Error(), tc.not)) {
			t.Errorf("%s: want %q, not %q, got %v", tc.sql, tc.want, tc.not, err)
		}
	}
}
