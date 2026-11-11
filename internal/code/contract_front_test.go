package code

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// frontWith writes the composable body (the call is on the line that holds CALL, marker removed) and extra files; it returns base, file and line.
func frontWith(t *testing.T, composable string, extra map[string]string) (string, string, int) {
	t.Helper()
	base := realDir(t)
	line := 0
	for i, l := range strings.Split(composable, "\n") {
		if strings.Contains(l, "CALL") {
			line = i + 1
		}
	}
	if line == 0 {
		t.Fatal("the composable needs a CALL marker")
	}
	files := map[string]string{"sales/orders/composables/useOrders.ts": strings.ReplaceAll(composable, "/*CALL*/", "")}
	for k, v := range extra {
		files[k] = v
	}
	writeTree(t, base, files)
	return base, filepath.Join(base, "sales/orders/composables/useOrders.ts"), line
}

func names(fs []sent) []string {
	var out []string
	for _, f := range fs {
		n := f.name
		if f.optional {
			n += "?"
		}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func TestSentInlineObject(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(rest) {\n    /*CALL*/await axios.post('orders', { a: 1, bC, ...rest })\n  }\n}\n", nil)
	got, unresolved := sentFields(base, file, line)
	if !reflect.DeepEqual(names(got), []string{"a", "bC"}) || len(unresolved) != 1 || !strings.Contains(unresolved[0], "...rest") {
		t.Fatalf("got %v, unresolved %v", names(got), unresolved)
	}
}

var formType = map[string]string{"sales/orders/types/Order.ts": "export type Form = {\n  name: string;\n  notes?: string | null;\n  lines: { qty: number }[];\n};\n"}

func TestSentSpreadTypedParam(t *testing.T) {
	base, file, line := frontWith(t, "import type { Form } from '../types/Order'\nexport function useOrders() {\n  async function save(q: string, info: Form) {\n    /*CALL*/await axios.post('orders', { ...info, customerId: q })\n  }\n}\n", formType)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"customerId", "lines", "name", "notes?"}; !reflect.DeepEqual(names(got), want) || len(unresolved) != 0 {
		t.Fatalf("got %v, unresolved %v; want %v", names(got), unresolved, want)
	}
}

func TestSentDirectTypedParam(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  async function update(id: string, info: Form) {\n    /*CALL*/await axios.put('orders/' + id, info)\n  }\n}\n", formType)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"lines", "name", "notes?"}; !reflect.DeepEqual(names(got), want) || len(unresolved) != 0 {
		t.Fatalf("got %v, unresolved %v", names(got), unresolved)
	}
}

func TestSentIntersectionType(t *testing.T) {
	types := map[string]string{"shared/types/Base.ts": "export interface Base {\n  a: string\n  b?: number\n}\n", "sales/orders/types/F.ts": "export type F = Base & {\n  extra: string;\n};\n"}
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(info: F) {\n    /*CALL*/await axios.post('orders', info)\n  }\n}\n", types)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"a", "b?", "extra"}; !reflect.DeepEqual(names(got), want) || len(unresolved) != 0 {
		t.Fatalf("got %v, unresolved %v", names(got), unresolved)
	}
}

func TestSentUnionType(t *testing.T) {
	types := map[string]string{"sales/orders/types/U.ts": "export type A = { x: string; y: string };\nexport type B = { x: string; z?: string };\n"}
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(info: A | B) {\n    /*CALL*/await axios.post('orders', { ...info })\n  }\n}\n", types)
	got, _ := sentFields(base, file, line)
	if want := []string{"x", "y?", "z?"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("got %v, want %v", names(got), want)
	}
}

func TestSentLocalVariable(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(info: Form, version: number) {\n    const payload = {\n      ...info,\n      version,\n    };\n    /*CALL*/const r = await axios.post('orders', payload);\n  }\n}\n", formType)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"lines", "name", "notes?", "version"}; !reflect.DeepEqual(names(got), want) || len(unresolved) != 0 {
		t.Fatalf("got %v, unresolved %v", names(got), unresolved)
	}
}

func TestSentFormData(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  async function upload(formData: FormData) {\n    /*CALL*/await axios.post('orders/upload', formData, { headers: {} })\n  }\n}\n", nil)
	got, unresolved := sentFields(base, file, line)
	if len(got) != 0 || len(unresolved) != 1 || !strings.Contains(unresolved[0], "formData") {
		t.Fatalf("got %v, unresolved %v", names(got), unresolved)
	}
}

func TestSentTypeNotFound(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(info: Missing) {\n    /*CALL*/await axios.post('orders', { ...info, a: 1 })\n  }\n}\n", nil)
	got, unresolved := sentFields(base, file, line)
	if !reflect.DeepEqual(names(got), []string{"a"}) || len(unresolved) != 1 || !strings.Contains(unresolved[0], "Missing") {
		t.Fatalf("got %v, unresolved %v", names(got), unresolved)
	}
}

func TestWireNameMatchesConverter(t *testing.T) {
	// expected values: snake-case@3.0.4, the dependency of axios-case-converter@1.1.1
	for in, want := range map[string]string{"quoteId": "quote_id", "saleTax": "sale_tax", "id": "id", "URLPath": "url_path", "address1": "address1", "version2Id": "version2_id", "HTMLParser": "html_parser", "already_snake": "already_snake", "perPage": "per_page"} {
		if got := wireName(in, true); got != want {
			t.Errorf("wireName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := wireName("quoteId", false); got != "quoteId" {
		t.Errorf("without the converter the name stays: %q", got)
	}
}

func TestCaseConverter(t *testing.T) {
	with, without := realDir(t), realDir(t)
	writeTree(t, with, map[string]string{"package.json": `{"dependencies": {"axios": "1", "axios-case-converter": "1.1.1"}}`})
	writeTree(t, without, map[string]string{"package.json": `{"dependencies": {"axios": "1"}, "description": "axios-case-converter is not used"}`})
	if !caseConverter(with) || caseConverter(without) || caseConverter(realDir(t)) {
		t.Fatal("the converter counts only as a dependency")
	}
}
