package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// checkSetup is a SQLite table (no primary key, so two rows can share an id) and a page tool holding the requests the screen "made".
func checkSetup(t *testing.T) (*checker, *pageTool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		"CREATE TABLE quotes (id TEXT, sub TEXT, office TEXT, notes TEXT, code TEXT, note2 TEXT)",
		"INSERT INTO quotes VALUES ('059', 'AI', 'MIA', 'secret note', '007', NULL)",
		"INSERT INTO quotes VALUES ('060', 'AI', 'MIA', 'ñandú 😀', '1', '')",
		"INSERT INTO quotes VALUES ('061', 'AI', 'MIA', 'x', '1', NULL)",
		"INSERT INTO quotes VALUES ('061', 'AI', 'MIA', 'y', '1', NULL)",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	p := newPage("http://app.test")
	return &checker{s: &source{url: "sqlite://" + path}, p: p}, p
}

func sent(p *pageTool, method, path, req, res string) {
	p.caps = append(p.caps, capture{M: method, U: "http://demo.localhost:8001" + path, S: 200, Req: req, Res: res})
}

func args(id string, m map[string]string) checkArgs {
	return checkArgs{Sent: "POST /api/quotes", Table: "quotes", Where: map[string]string{"id": id}, Map: m}
}

var fourFields = map[string]string{"sub_type": "sub", "office": "office", "notes": "notes", "code": "code"}

func run(t *testing.T, c *checker, a checkArgs) (string, error) {
	t.Helper()
	return c.run(context.Background(), a)
}

func TestCheckSaysEqualWithoutShowingTheValues(t *testing.T) {
	c, p := checkSetup(t)
	sent(p, "POST", "/api/quotes", `{"sub_type":"AI","office":"MIA","notes":"secret note","code":"007"}`, "")
	out, err := run(t, c, args("059", fourFields))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"4 fields", "4 equal", "0 different"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q: %q", want, out)
		}
	}
	if strings.Contains(out, "secret note") || strings.Contains(out, "007") {
		t.Errorf("the values of fields that are equal must not be shown (only the rows that differ): %q", out)
	}
}

func TestCheckShowsBothValuesWhereTheyDifferAndKeepsTheLeadingZeros(t *testing.T) {
	c, p := checkSetup(t)
	sent(p, "POST", "/api/quotes", `{"sub_type":"AI","office":"MIA","notes":"secret note","code":"7"}`, "")
	out, err := run(t, c, args("059", fourFields))
	if err != nil || !strings.Contains(out, "1 different") || !strings.Contains(out, "code\t7\t007") {
		t.Errorf("%q, %v", out, err)
	}
	sent(p, "POST", "/api/quotes", `{"sub_type":"AI","office":"MIA","notes":"secret note","code":7}`, "")
	out, err = run(t, c, args("059", fourFields))
	if err != nil || !strings.Contains(out, "code\t7\t007") {
		t.Errorf("a number 7 and the text 007 are not the same: %q, %v", out, err)
	}
}

func TestCheckCountsAnEmptyAndANullAsEquivalentButSaysSo(t *testing.T) {
	c, p := checkSetup(t)
	m := map[string]string{"n": "note2"}
	for _, tc := range []struct{ id, body string }{
		{"059", `{"n":null}`}, // NULL stored, null sent
		{"059", `{"n":""}`},   // NULL stored, empty sent
		{"060", `{"n":null}`}, // empty stored, null sent
	} {
		sent(p, "POST", "/api/quotes", tc.body, "")
		out, err := run(t, c, args(tc.id, m))
		if err != nil || !strings.Contains(out, "0 different") || !strings.Contains(out, "1 empty~") {
			t.Errorf("%s %s: %q, %v", tc.id, tc.body, out, err)
		}
	}
}

func TestCheckCountsAnEmptyThatTheScreenReadBackAsNullAsEquivalentToo(t *testing.T) {
	c, p := checkSetup(t)
	sent(p, "POST", "/api/quotes", `{"n":""}`, "")
	sent(p, "GET", "/api/quotes/060", "", `{"quote":{"n":null}}`)
	a := args("060", map[string]string{"n": "note2"})
	a.After, a.At = "GET /api/quotes/060", "quote"
	out, err := run(t, c, a) // stored is '' and sent is '': equal to each other, but the screen read null
	if err != nil || !strings.Contains(out, "1 empty~") || !strings.Contains(out, "0 different") {
		t.Errorf("%q, %v", out, err)
	}
}

func TestCheckComparesNonASCIIAndAMissingField(t *testing.T) {
	c, p := checkSetup(t)
	sent(p, "POST", "/api/quotes", `{"notes":"ñandú 😀"}`, "")
	out, err := run(t, c, args("060", map[string]string{"notes": "notes", "missing": "code"}))
	if err != nil || !strings.Contains(out, "1 equal") || !strings.Contains(out, "missing\t(absent)\t1") {
		t.Errorf("%q, %v", out, err)
	}
}

func TestCheckComparesWhatTheScreenReadAfterSaving(t *testing.T) {
	c, p := checkSetup(t)
	sent(p, "POST", "/api/quotes", `{"sub_type":"AI","office":"MIA"}`, "")
	sent(p, "GET", "/api/quotes/059", "", `{"quote":{"sub_type":"AI","office":"Miami"}}`)
	a := args("059", map[string]string{"sub_type": "sub", "office": "office"})
	a.After, a.At = "GET /api/quotes/059", "quote"
	out, err := run(t, c, a)
	if err != nil || !strings.Contains(out, "office\tMIA\tMIA\tMiami") || !strings.Contains(out, "1 different") {
		t.Errorf("%q, %v", out, err)
	}
	a.At = "data.0"
	sent(p, "GET", "/api/quotes", "", `{"data":[{"sub_type":"AI","office":"MIA"}]}`)
	a.After = "GET /api/quotes"
	if out, err = run(t, c, a); err != nil || !strings.Contains(out, "0 different") {
		t.Errorf("a path with an index: %q, %v", out, err)
	}
}

func TestCheckRefusesWhatItCannotCompareHonestly(t *testing.T) {
	c, p := checkSetup(t)
	if _, err := run(t, c, args("059", fourFields)); err == nil || !strings.Contains(err.Error(), "no captured request matches") {
		t.Errorf("nothing captured: %v", err)
	}
	sent(p, "POST", "/api/quotes", clip(`{"notes":"`+strings.Repeat("a", storeClip+500)+`"}`, storeClip), "")
	if _, err := run(t, c, args("059", map[string]string{"notes": "notes"})); err == nil || !strings.Contains(err.Error(), "cut") {
		t.Errorf("a body that was cut must not be compared as if it were whole: %v", err)
	}
	sent(p, "POST", "/api/quotes", `{"notes":"x"}`, "")
	sent(p, "GET", "/api/quotes", "", `{"data":[{"notes":"x"…(+612 chars)`) // the mark that page.js leaves when it cuts a body
	cutRead := args("059", map[string]string{"notes": "notes"})
	cutRead.After, cutRead.At = "GET /api/quotes", "data.0"
	if _, err := run(t, c, cutRead); err == nil || !strings.Contains(err.Error(), "cut") {
		t.Errorf("a body that page.js cut must be named as cut, not as not JSON: %v", err)
	}
	p.caps = p.caps[:len(p.caps)-1]
	if _, err := run(t, c, args("999", fourFields)); err == nil || !strings.Contains(err.Error(), "no row") {
		t.Errorf("no row: %v", err)
	}
	if _, err := run(t, c, args("061", fourFields)); err == nil || !strings.Contains(err.Error(), "more than one row") {
		t.Errorf("two rows: %v", err)
	}
	if _, err := run(t, c, args("x' OR '1'='1", fourFields)); err == nil || !strings.Contains(err.Error(), "no row") {
		t.Errorf("the value of where is data, not SQL: %v", err)
	}
}

func TestCheckRefusesNamesThatAreNotColumns(t *testing.T) {
	c, p := checkSetup(t)
	sent(p, "POST", "/api/quotes", `{"notes":"x"}`, "")
	bad := args("059", map[string]string{"notes": "notes"})
	bad.Table = "quotes; DROP TABLE quotes"
	if _, err := run(t, c, bad); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Errorf("table: %v", err)
	}
	bad = args("059", map[string]string{"notes": "notes) --"})
	if _, err := run(t, c, bad); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Errorf("column in map: %v", err)
	}
	bad = args("059", map[string]string{"notes": "notes"})
	bad.Where = map[string]string{"id = 1 OR 1": "x"}
	if _, err := run(t, c, bad); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Errorf("column in where: %v", err)
	}
	if out, err := run(t, c, checkArgs{Sent: "POST /api/quotes", Table: "quotes", Where: map[string]string{"id": "059"}}); err == nil {
		t.Errorf("without a map there is nothing to compare: %q", out)
	}
}

func TestCheckAnswerFitsTheBudgetEvenWhenEverythingDiffers(t *testing.T) {
	c, p := checkSetup(t)
	m, body := map[string]string{}, []string{}
	for _, col := range []string{"sub", "office", "notes", "code", "note2"} {
		m["field_"+col] = col
		body = append(body, `"field_`+col+`":"`+strings.Repeat("z", 90)+`"`)
	}
	sent(p, "POST", "/api/quotes", "{"+strings.Join(body, ",")+"}", "")
	out, err := run(t, c, args("059", m))
	if err != nil || len(out) > 1500 || !strings.Contains(out, "5 different") {
		t.Errorf("%d bytes: %q, %v", len(out), out, err)
	}
	if strings.Contains(out, strings.Repeat("z", 60)) {
		t.Errorf("a value must be cut at 40 characters: %q", out)
	}
}

func TestCheckSaysWhenOnlyTheSpacesDiffer(t *testing.T) {
	c, p := checkSetup(t)
	sent(p, "POST", "/api/quotes", `{"notes":"  a  \t  b "}`, "")
	db := writeDBFor(t, c)
	if _, err := db.Exec("UPDATE quotes SET notes = 'a b' WHERE id = '059'"); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, c, args("059", map[string]string{"notes": "notes"}))
	if err != nil || strings.Contains(out, "1 equal") || !strings.Contains(out, "1 space~") {
		t.Errorf("a text that the app trimmed or collapsed is not equal: %q, %v", out, err)
	}
}

func TestCheckComparesOnlyASaveThatSucceededAndAReadThatCameAfterIt(t *testing.T) {
	c, p := checkSetup(t)
	p.caps = append(p.caps, capture{M: "POST", U: "http://demo.localhost:8001/api/quotes", S: 422, Req: `{"notes":"x"}`})
	if _, err := run(t, c, args("059", map[string]string{"notes": "notes"})); err == nil || !strings.Contains(err.Error(), "422") {
		t.Errorf("a save that answered 422 saved nothing: %v", err)
	}
	p.caps = nil
	sent(p, "GET", "/api/quotes/059", "", `{"quote":{"notes":"old"}}`) // a read from BEFORE the save
	sent(p, "POST", "/api/quotes", `{"notes":"x"}`, "")
	a := args("059", map[string]string{"notes": "notes"})
	a.After, a.At = "GET /api/quotes/059", "quote"
	if _, err := run(t, c, a); err == nil || !strings.Contains(err.Error(), "after") {
		t.Errorf("a read that came before the save must not be used as what the screen read after it: %v", err)
	}
	p.caps = nil
	sent(p, "POST", "/api/quotes-extra", `{"notes":"x"}`, "")
	if _, err := run(t, c, args("059", map[string]string{"notes": "notes"})); err == nil {
		t.Error("POST /api/quotes must not match /api/quotes-extra")
	}
}
