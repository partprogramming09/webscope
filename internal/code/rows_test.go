package code

import (
	"strings"
	"testing"
)

// rowsSetup is the quotes setup plus two rows of one table that two systems wrote, and their child rows.
func rowsSetup(t *testing.T) *Tool {
	t.Helper()
	c := codeSetup(t)
	conn := writeDB(t, c)
	for _, q := range []string{
		"CREATE TABLE q2 (id TEXT, estado TEXT, motivo TEXT, usuario TEXT, nota TEXT, monto TEXT, creado TEXT, espacios TEXT, vacio TEXT)",
		"INSERT INTO q2 VALUES ('072', 'A', 'ASIGNA', 'Ana', 'SECRET_NOTE', '7', 't1', 'dos', NULL)",
		"INSERT INTO q2 VALUES ('073', 'A', '', 'ANA', NULL, '07', 't2', ' dos ', '')",
		"INSERT INTO q2 VALUES ('074', 'A', '', 'ANA', NULL, '07', 't2', ' dos ', '')",
		"INSERT INTO q2 VALUES ('074', 'B', '', 'ANA', NULL, '07', 't2', ' dos ', '')",
		"CREATE TABLE q2_child (qid TEXT, x TEXT)",
		"INSERT INTO q2_child VALUES ('072', 'a'), ('072', 'b'), ('072', 'c'), ('073', 'a'), ('073', 'b')",
	} {
		if _, err := conn.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func TestRowsSaysHowTwoRowsDifferAndNeverPrintsAValue(t *testing.T) {
	out, err := code(t, rowsSetup(t), "rows", map[string]any{
		"table": "q2", "a": map[string]any{"id": "072"}, "b": map[string]any{"id": "073"},
		"children": map[string]any{"q2_child": "qid"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"q2", "8 columns", "1 equal", "7 differ",
		"motivo: a filled, b empty", "usuario: case only", "nota: a filled, b NULL", "monto: same number, other text",
		"espacios: spaces only", "vacio: a NULL, b empty", "creado: differs",
		"q2_child: a 3, b 2 ≠",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	for _, not := range []string{"ASIGNA", "SECRET_NOTE", "Ana", "estado:", "id:", "t1", "t2"} {
		if strings.Contains(out, not) {
			t.Errorf("must not contain %q (a value, an equal column or the key):\n%s", not, out)
		}
	}
}

func TestRowsDoesNotCompareTheKeyWhenTheDatabaseNamesItInOtherCase(t *testing.T) {
	c := rowsSetup(t)
	if _, err := writeDB(t, c).Exec("CREATE TABLE q3 (COTIZACION_ID TEXT, estado TEXT); INSERT INTO q3 VALUES ('1', 'A'); INSERT INTO q3 VALUES ('2', 'B')"); err != nil {
		t.Fatal(err)
	}
	out, err := code(t, c, "rows", map[string]any{"table": "q3", "a": map[string]any{"cotizacion_id": "1"}, "b": map[string]any{"cotizacion_id": "2"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "COTIZACION_ID:") || !strings.Contains(out, "1 columns, 0 equal, 1 differ") {
		t.Errorf("the column that picks the rows is left out, whatever its case:\n%s", out)
	}
}

func TestRowsRefusesWhatItCannotCompare(t *testing.T) {
	c := rowsSetup(t)
	a, b := map[string]any{"id": "072"}, map[string]any{"id": "073"}
	for name, args := range map[string]map[string]any{
		"no table":      {"a": a, "b": b},
		"bad table":     {"table": "q2; DROP TABLE q2", "a": a, "b": b},
		"no b":          {"table": "q2", "a": a},
		"bad column":    {"table": "q2", "a": map[string]any{"id`": "1"}, "b": b},
		"no row":        {"table": "q2", "a": map[string]any{"id": "999"}, "b": b},
		"two rows":      {"table": "q2", "a": a, "b": map[string]any{"id": "074"}},
		"bad child":     {"table": "q2", "a": a, "b": b, "children": map[string]any{"q2_child": "q id"}},
		"unknown table": {"table": "nope", "a": a, "b": b},
	} {
		if out, err := code(t, c, "rows", args); err == nil {
			t.Errorf("%s: want an error, got %q", name, out)
		}
	}
	if _, err := code(t, c, "rows", map[string]any{"table": "q2", "a": map[string]any{"id": "074"}, "b": b}); err == nil || !strings.Contains(err.Error(), "more than one row") {
		t.Errorf("two rows must say so and ask for another column in a, got %v", err)
	}
}

func TestRowsOfTwoEqualRowsSaysSo(t *testing.T) {
	out, err := code(t, rowsSetup(t), "rows", map[string]any{"table": "q2", "a": map[string]any{"id": "072"}, "b": map[string]any{"id": "072"}})
	if err != nil || !strings.Contains(out, "no differences") {
		t.Errorf("%q, %v", out, err)
	}
}

func TestLeftoversCountsTheRowsATestLeftBehindWithoutShowingThem(t *testing.T) {
	c := codeSetup(t)
	conn := writeDB(t, c)
	for _, q := range []string{
		"CREATE TABLE lo (id TEXT, code TEXT)",
		"INSERT INTO lo VALUES ('1', 'ZZ_A'), ('2', 'ZZ_B'), ('3', 'ZZXC'), ('4', 'real'), ('5', NULL)",
	} {
		if _, err := conn.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	out, err := code(t, c, "leftovers", map[string]any{"table": "lo", "column": "code"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2 rows") || strings.Contains(out, "ZZ_A") || strings.Contains(out, "real") {
		t.Errorf("only ZZ_ rows, counted and not shown (the _ is not a wildcard): %q", out)
	}
	if out, _ := code(t, c, "leftovers", map[string]any{"table": "lo", "column": "code", "prefix": "NOPE_"}); !strings.Contains(out, "0 rows") {
		t.Errorf("none left must say 0 rows: %q", out)
	}
	if _, err := code(t, c, "leftovers", map[string]any{"table": "lo; DROP", "column": "code"}); err == nil {
		t.Error("an invalid table name must be refused")
	}
	if _, err := code(t, c, "leftovers", map[string]any{"table": "lo", "column": "code", "prefix": "a'b"}); err == nil {
		t.Error("an invalid prefix must be refused")
	}
}
