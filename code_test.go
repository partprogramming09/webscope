package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// codeSetup is a small Laravel-like tree with traps: a .env that names the column, a vendor folder, a symlinked file and a symlinked folder that
// point outside the root, and a SQLite table. outside holds a file with the column that must never be listed.
func codeSetup(t *testing.T) *coder {
	t.Helper()
	root, outside := t.TempDir(), t.TempDir()
	write := func(base, rel, body string) {
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(root, "app/Models/Quote.php", "<?php\nclass Quote extends Model {\n    protected $table = 'quotes';\n}\n")
	write(root, "app/Services/QuoteService.php", "<?php\nclass QuoteService {\n  function show(Quote $q) {\n    return $q->estado;\n  }\n  function close($q) {\n\n    $q->update(['estado' => 'BORRADO']);\n\n  }\n}\n")
	write(root, "app/Http/QuoteResource.php", "<?php // resource of a Quote\nreturn ['estado' => $this->estado, 'code' => $this->code];\n") // reads only
	write(root, "app/Other.php", "<?php\n$x = 'estado'; // does not name the table or the model\n")
	write(root, ".env", "DB_PASSWORD=hunter2\nestado='SECRET_IN_ENV'\n")
	write(root, "config/.env.php", "<?php 'estado' // quotes SECRET_IN_DOTENV_PHP\n")
	write(root, "vendor/lib/Quote.php", "<?php // quotes 'estado' SECRET_IN_VENDOR\n")
	write(root, "storage/framework/Quote.php", "<?php // quotes 'estado' SECRET_IN_STORAGE\n")
	write(outside, "leak.php", "<?php // quotes 'estado' SECRET_OUTSIDE\n")
	write(outside, "dir/leak2.php", "<?php // quotes 'estado' SECRET_OUTSIDE_DIR\n")
	if err := os.Symlink(filepath.Join(outside, "leak.php"), filepath.Join(root, "app/link.php")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "dir"), filepath.Join(root, "app/linkdir")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		"CREATE TABLE quotes (id TEXT, estado TEXT NOT NULL DEFAULT 'X', code TEXT, empty_col TEXT, nota TEXT)",
		"INSERT INTO quotes VALUES ('1', 'A', '007', NULL, 'SECRET_CELL_VALUE')",
		"INSERT INTO quotes VALUES ('2', 'B', '', NULL, NULL)",
		"INSERT INTO quotes VALUES ('3', 'C', NULL, NULL, '')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return &coder{s: &source{url: "sqlite://" + path}, backend: root}
}

func code(t *testing.T, c *coder, action string, args map[string]any) (string, error) {
	t.Helper()
	return c.run(context.Background(), codeArgs{Action: action, Args: args})
}

func TestConsumersListsReadersAndWritersWithFileAndLine(t *testing.T) {
	out, err := code(t, codeSetup(t), "consumers", map[string]any{"table": "quotes", "column": "estado"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"model: Quote", "app/Services/QuoteService.php:4", "READS/WRITES", "app/Http/QuoteResource.php:2", "READS "} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Other.php") {
		t.Errorf("a file that names neither the table nor its model must not be searched:\n%s", out)
	}
	if !strings.Contains(out, "estimate") {
		t.Errorf("the answer must say it is an estimate:\n%s", out)
	}
}

func TestConsumersNeverReadsOutsideTheRootNorEnvNorVendor(t *testing.T) {
	c := codeSetup(t)
	for _, args := range []map[string]any{{"table": "quotes", "column": "estado"}, {"column_only": "estado"}} {
		out, err := code(t, c, "consumers", args)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"SECRET_IN_ENV", "hunter2", "SECRET_IN_DOTENV_PHP", "SECRET_IN_VENDOR", "SECRET_IN_STORAGE", "SECRET_OUTSIDE", "link.php", "linkdir", ".env"} {
			if strings.Contains(out, secret) {
				t.Errorf("%v: the answer shows %q:\n%s", args, secret, out)
			}
		}
	}
}

func TestConsumersTreatsTheColumnAsTextNotAsARegularExpression(t *testing.T) {
	c := codeSetup(t)
	out, err := code(t, c, "consumers", map[string]any{"table": "quotes", "column": "est.do"})
	if err != nil || strings.Contains(out, "QuoteService.php") {
		t.Errorf("a dot is a dot: %q, %v", out, err)
	}
	if _, err = code(t, c, "consumers", map[string]any{"table": "quotes", "column": "(["}); err != nil && strings.Contains(err.Error(), "regexp") {
		t.Errorf("a column with regular expression characters must not break: %v", err)
	}
}

func TestConsumersRefusesWhatItCannotSearch(t *testing.T) {
	c := codeSetup(t)
	if _, err := code(t, c, "consumers", map[string]any{"table": "nope", "column": "estado"}); err == nil || !strings.Contains(err.Error(), "no backend file uses the table nope") {
		t.Errorf("a table nobody uses: %v", err)
	}
	if _, err := code(t, c, "consumers", map[string]any{"table": "quotes; DROP", "column": "estado"}); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Errorf("the table name: %v", err)
	}
	if _, err := code(t, &coder{s: c.s}, "consumers", map[string]any{"table": "quotes", "column": "estado"}); err == nil || !strings.Contains(err.Error(), "WEBSCOPE_BACKEND") {
		t.Errorf("without WEBSCOPE_BACKEND: %v", err)
	}
	if out, err := code(t, c, "consumers", map[string]any{"table": "quotes", "column": "id"}); err != nil || !strings.Contains(out, "short") {
		t.Errorf("a short column is searched with a warning: %q, %v", out, err)
	}
}

func TestFieldsGivesTypeNullCountsAndWhoUsesEachColumnButNeverAValue(t *testing.T) {
	out, err := code(t, codeSetup(t), "fields", map[string]any{"table": "quotes"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"quotes: 3 rows", "model Quote", "estado\tTEXT\tNO\t", "3/3", "code\tTEXT\tYES\t", "1/3", "empty_col", "0/3", "no value in any row", "nota\t", "nobody names it"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "SECRET_CELL_VALUE") || strings.Contains(out, "007") {
		t.Errorf("fields shows counts, never a stored value:\n%s", out)
	}
	if !strings.Contains(out, "estado\tTEXT\tNO\t'X'\t3/3\t1 / 1") {
		t.Errorf("estado is read by QuoteResource and written (and read) by QuoteService; the writer file is not counted again as reader:\n%s", out)
	}
}

func TestFieldsWorksWithoutTheBackendAndSaysSo(t *testing.T) {
	c := codeSetup(t)
	c.backend = ""
	out, err := code(t, c, "fields", map[string]any{"table": "quotes"})
	if err != nil || !strings.Contains(out, "WEBSCOPE_BACKEND") || !strings.Contains(out, "estado") {
		t.Errorf("%q, %v", out, err)
	}
	if _, err := code(t, c, "fields", map[string]any{"table": "no_such_table"}); err == nil {
		t.Error("a table that does not exist must fail")
	}
	if _, err := code(t, c, "fields", map[string]any{"table": "quotes) --"}); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Errorf("the table name: %v", err)
	}
}

func TestCodeRefusesAnUnknownActionAndNamesTheList(t *testing.T) {
	if _, err := code(t, codeSetup(t), "run", nil); err == nil || !strings.Contains(err.Error(), "fields") || !strings.Contains(err.Error(), "consumers") {
		t.Errorf("%v", err)
	}
}

func TestFieldsCutsAnAnswerThatIsTooBigAndSaysSo(t *testing.T) {
	c := codeSetup(t)
	path := strings.TrimPrefix(c.s.url, "sqlite://")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cols := make([]string, 400)
	for i := range cols {
		cols[i] = fmt.Sprintf("column_number_%03d_with_a_long_name TEXT", i)
	}
	if _, err := db.Exec("CREATE TABLE wide (" + strings.Join(cols, ", ") + ")"); err != nil {
		t.Fatal(err)
	}
	out, err := code(t, c, "fields", map[string]any{"table": "wide"})
	if err != nil || len(out) > maxBytes+200 || !strings.Contains(out, "columns shown") {
		t.Errorf("%d bytes: %v\n%.300s", len(out), err, out)
	}
}
