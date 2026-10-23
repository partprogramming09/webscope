package main

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// fixture writes a SQLite file with 122 rows in items and returns a source that opens it read-only.
func fixture(t *testing.T) *source {
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
	return &source{url: "sqlite://" + path}
}

func ask(t *testing.T, s *source, q string, limit int) (string, error) {
	t.Helper()
	return s.answer(context.Background(), queryArgs{SQL: q, Limit: limit})
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
	if len(out) > maxBytes+200 || !strings.Contains(out, "cut at 8000 bytes") {
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
	if _, err := ask(t, &source{}, "SELECT 1", 0); err == nil || !strings.Contains(err.Error(), "WEBSCOPE_DB_URL") {
		t.Errorf("unset: %v", err)
	}
	if _, err := ask(t, &source{url: "oracle://u:p@h/db"}, "SELECT 1", 0); err == nil || !strings.Contains(err.Error(), "mysql or sqlite") {
		t.Errorf("unsupported: %v", err)
	}
}

func TestThePasswordNeverAppearsInAnError(t *testing.T) {
	_, err := ask(t, &source{url: "mysql://user:s3cr3t-pw@127.0.0.1:1/db"}, "SELECT 1", 0)
	if err == nil {
		t.Fatal("a closed port gave no error")
	}
	if strings.Contains(err.Error(), "s3cr3t-pw") {
		t.Fatalf("the password leaked: %v", err)
	}
	if got := redact("denied for s3cr3t-pw and again s3cr3t-pw", "s3cr3t-pw"); strings.Contains(got, "s3cr3t-pw") || got != "denied for *** and again ***" {
		t.Fatalf("redact = %q", got)
	}
}
