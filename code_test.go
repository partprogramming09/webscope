package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
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

// writeDB is a writable connection to the SQLite file of c: the tool itself only reads, the test plays the part of "an action on the screen".
func writeDB(t *testing.T, c *coder) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", strings.TrimPrefix(c.s.url, "sqlite://"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSnapSaysNothingChangedWhenNothingDid(t *testing.T) {
	c := codeSetup(t)
	out, err := code(t, c, "snap", map[string]any{"name": "q", "sql": "SELECT id, estado, nota FROM quotes", "key": "id"})
	if err != nil || !strings.Contains(out, "3 rows") {
		t.Fatalf("%q, %v", out, err)
	}
	if out, err = code(t, c, "snap_diff", map[string]any{"name": "q"}); err != nil || !strings.Contains(out, "no changes") {
		t.Errorf("%q, %v", out, err)
	}
}

func TestSnapShowsNewDeletedAndChangedRowsWithBeforeAndAfter(t *testing.T) {
	c := codeSetup(t)
	if _, err := code(t, c, "snap", map[string]any{"name": "q", "sql": "SELECT id, estado, nota, code FROM quotes", "key": "id"}); err != nil {
		t.Fatal(err)
	}
	db := writeDB(t, c)
	for _, q := range []string{
		"UPDATE quotes SET estado = 'Z' WHERE id = '1'",
		"UPDATE quotes SET nota = '' WHERE id = '2'", // NULL to empty: a change, and it must be visible
		"DELETE FROM quotes WHERE id = '3'",
		"INSERT INTO quotes (id, estado, nota, code) VALUES ('4', 'N', 'ñandú 😀', '9')",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	out, err := code(t, c, "snap_diff", map[string]any{"name": "q"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"3 rows before, 3 now", "changed 1: estado A → Z", "changed 2: nota \\N → ''", "deleted 3", "new 4:", "ñandú 😀"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "code 007") || strings.Contains(out, "changed 1: estado A → Z, code") {
		t.Errorf("a column that did not change must not be listed:\n%s", out)
	}
}

func TestSnapRefusesWhatItCannotCompareNorKeepSafely(t *testing.T) {
	c := codeSetup(t)
	if _, err := code(t, c, "snap", map[string]any{"name": "q", "sql": "DELETE FROM quotes"}); err == nil {
		t.Error("a statement that is not a read must be refused")
	}
	if _, err := code(t, c, "snap", map[string]any{"name": "q", "sql": "SELECT estado FROM quotes", "key": "estado"}); err != nil {
		t.Errorf("distinct values are a valid key: %v", err)
	}
	if _, err := code(t, c, "snap", map[string]any{"name": "dup", "sql": "SELECT id, estado FROM quotes UNION ALL SELECT id, estado FROM quotes"}); err == nil || !strings.Contains(err.Error(), "not unique") {
		t.Errorf("a key that repeats: %v", err)
	}
	if _, err := code(t, c, "snap", map[string]any{"name": "bad name!", "sql": "SELECT id FROM quotes"}); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Errorf("the name: %v", err)
	}
	if _, err := code(t, c, "snap", map[string]any{"name": "k", "sql": "SELECT id FROM quotes", "key": "nope"}); err == nil || !strings.Contains(err.Error(), "key") {
		t.Errorf("a key that is not a column: %v", err)
	}
	if _, err := code(t, c, "snap_diff", map[string]any{"name": "missing"}); err == nil || !strings.Contains(err.Error(), "q") {
		t.Errorf("an unknown snapshot names the known ones: %v", err)
	}
}

func TestSnapRefusesAResultTooBigToKeepAndCutsAnAnswerThatIsTooBig(t *testing.T) {
	c := codeSetup(t)
	db := writeDB(t, c)
	tx, _ := db.Begin()
	for i := 0; i < maxSnapRows+1; i++ {
		tx.Exec("INSERT INTO quotes (id, estado) VALUES (?, 'B')", fmt.Sprintf("big%05d", i))
	}
	tx.Commit()
	if _, err := code(t, c, "snap", map[string]any{"name": "all", "sql": "SELECT id, estado FROM quotes"}); err == nil || !strings.Contains(err.Error(), "WHERE") {
		t.Errorf("too many rows must say to narrow the query: %v", err)
	}
	if _, err := code(t, c, "snap", map[string]any{"name": "some", "sql": "SELECT id, estado FROM quotes WHERE id LIKE 'big%'", "key": "id"}); err == nil {
		// still too many rows (maxSnapRows+1): the same refusal
		t.Error("the filtered query has more rows than the limit")
	}
	if _, err := code(t, c, "snap", map[string]any{"name": "few", "sql": "SELECT id, estado, nota FROM quotes WHERE id IN ('1','2','3')", "key": "id"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE quotes SET nota = ? WHERE id = '1'", strings.Repeat("y", 500)); err != nil {
		t.Fatal(err)
	}
	out, _ := code(t, c, "snap_diff", map[string]any{"name": "few"})
	if strings.Contains(out, strings.Repeat("y", 60)) {
		t.Errorf("a value must be cut at 40 characters:\n%s", out)
	}
}

func TestAModelWithAnOddFileNameDoesNotBreakTheSearch(t *testing.T) {
	c := codeSetup(t)
	for rel, body := range map[string]string{"app/Models/Foo (old.php": "<?php\nclass Foo { protected $table = 'quotes'; }\n", "app/Models/Dq.php": "<?php\nclass Dq { protected $table = \"quotes\"; }\n", "app/Models/.php": "<?php\n$table = 'quotes';\n"} {
		if err := os.WriteFile(filepath.Join(c.backend, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := code(t, c, "consumers", map[string]any{"table": "quotes", "column": "estado"})
	if err != nil || !strings.Contains(out, "Dq") || strings.Contains(out, "Other.php") {
		t.Errorf("%q, %v", out, err)
	}
	if out, err = code(t, c, "fields", map[string]any{"table": "quotes"}); err != nil || !strings.Contains(out, "Dq") {
		t.Errorf("%q, %v", out, err)
	}
}

// writeDBFor is a writable connection to the SQLite file behind c.s, for the tests of other tools.
func writeDBFor(t *testing.T, c *checker) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", strings.TrimPrefix(c.s.url, "sqlite://"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSnapSeesAChangeThatIsFarInsideAVeryLongValue(t *testing.T) {
	c := codeSetup(t)
	db := writeDB(t, c)
	long := strings.Repeat("a", 20000)
	if _, err := db.Exec("UPDATE quotes SET nota = ? WHERE id = '1'", long+"X"); err != nil {
		t.Fatal(err)
	}
	if _, err := code(t, c, "snap", map[string]any{"name": "L", "sql": "SELECT id, nota FROM quotes WHERE id = '1'", "key": "id"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE quotes SET nota = ? WHERE id = '1'", long+"Y"); err != nil {
		t.Fatal(err)
	}
	out, err := code(t, c, "snap_diff", map[string]any{"name": "L"})
	if err != nil || !strings.Contains(out, "changed 1") {
		t.Errorf("only the last character changed and snap kept a cut value: %q, %v", out, err)
	}
	if len(out) > 1000 {
		t.Errorf("the answer is %d bytes", len(out))
	}
}

func TestTheWalkSkipsWhatIsNotARegularFileAndTheNamesInAnyCase(t *testing.T) {
	c := codeSetup(t)
	if err := syscall.Mkfifo(filepath.Join(c.backend, "app/Models/Pipe.php"), 0o644); err != nil { // reading it would block for ever
		t.Skip("no fifo here:", err)
	}
	done := make(chan string, 1)
	go func() {
		out, _ := code(t, c, "consumers", map[string]any{"table": "quotes", "column": "estado"})
		done <- out
	}()
	select {
	case out := <-done:
		if !strings.Contains(out, "QuoteService.php") {
			t.Errorf("the search did not finish well: %q", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a fifo named Pipe.php blocked the search")
	}
}

func TestFileOfRefusesALinkInsideTheRootThatLeadsToASkippedFolder(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{"storage/framework/S.php": "<?php // SECRET_STORAGE\n", "vendor/v/V.php": "<?php // SECRET_VENDOR\n", "app/Models/Ok.php": "<?php\n"} {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	os.Symlink(filepath.Join(root, "storage/framework"), filepath.Join(root, "app/Store"))
	os.Symlink(filepath.Join(root, "vendor"), filepath.Join(root, "app/Lib"))
	os.WriteFile(filepath.Join(root, "app/Models/.ENV.php"), []byte("<?php\n"), 0o644)
	real, _ := filepath.EvalSymlinks(root)
	for _, cls := range []string{`App\Store\S`, `App\Lib\v\V`, `Vendor\v\V`, `App\Models\.ENV`} {
		if f := fileOf(real, cls); f != "" {
			t.Errorf("%s resolved to %s", cls, f)
		}
	}
	if fileOf(real, `App\Models\Ok`) == "" {
		t.Error("a plain model must resolve")
	}
}

func TestConsumersByTableSaysWhichFilesItSearchedAndHowToSearchAll(t *testing.T) {
	out, err := code(t, codeSetup(t), "consumers", map[string]any{"table": "quotes", "column": "estado"})
	if err != nil || !strings.Contains(out, "column_only") || !strings.Contains(out, "NOTE: only the files that name") {
		t.Errorf("a file that reads $this->column without naming the table is not searched, and the answer must say so: %q, %v", out, err)
	}
	out, err = code(t, codeSetup(t), "consumers", map[string]any{"column_only": "estado"})
	if err != nil || strings.Contains(out, "NOTE: only the files") {
		t.Errorf("column_only searches everything, so it has no such note: %q, %v", out, err)
	}
}
