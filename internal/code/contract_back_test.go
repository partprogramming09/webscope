package code

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTree writes files (path relative to base -> body) under base.
func writeTree(t *testing.T, base string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// realDir is a temp folder by its real path (fileOf compares real paths; on macOS the temp folder is behind a link).
func realDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

const orderCtl = "app/Modules/Sales/Orders/Controllers/OrderController.php"

// backWithRequest is a backend whose OrderController::store takes StoreOrderRequest, with rulesBody as the body of rules().
func backWithRequest(t *testing.T, rulesBody string) string {
	root := realDir(t)
	writeTree(t, root, map[string]string{
		orderCtl: "<?php\nnamespace App\\Modules\\Sales\\Orders\\Controllers;\n\nuse App\\Modules\\Sales\\Orders\\Requests\\StoreOrderRequest;\n\nclass OrderController\n{\n    public function store(StoreOrderRequest $request)\n    {\n        return $this->service->create($request->validated());\n    }\n\n    public function index() {}\n}\n",
		"app/Modules/Sales/Orders/Requests/StoreOrderRequest.php": "<?php\nnamespace App\\Modules\\Sales\\Orders\\Requests;\n\nclass StoreOrderRequest extends FormRequest\n{\n    public function rules(): array\n    {\n" + rulesBody + "\n    }\n}\n",
	})
	return root
}

func ruleByName(rs ruleSet, name string) (rule, bool) {
	for _, r := range rs.rules {
		if r.name == name {
			return r, true
		}
	}
	return rule{}, false
}

func TestRulesArrayAndPipeForms(t *testing.T) {
	root := backWithRequest(t, "        return [\n            'a' => ['required', 'integer'],\n            'b' => 'nullable|string',\n            'c' => 'required_if:x,1',\n        ];")
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if err != nil {
		t.Fatal(err)
	}
	if rs.source != "StoreOrderRequest" || !rs.literal || len(rs.rules) != 3 {
		t.Fatalf("got %+v", rs)
	}
	a, _ := ruleByName(rs, "a")
	b, _ := ruleByName(rs, "b")
	c, _ := ruleByName(rs, "c")
	if !a.required || a.nullable || b.required || !b.nullable || c.required {
		t.Fatalf("a=%+v b=%+v c=%+v", a, b, c)
	}
	if a.line != 9 { // the line of 'a' in StoreOrderRequest.php
		t.Fatalf("a.line = %d, want 9", a.line)
	}
}

func TestRulesNestedKeysCollapse(t *testing.T) {
	root := backWithRequest(t, "        return [\n            'items' => ['array'],\n            'items.*.qty' => ['required'],\n            'tags.0' => 'string',\n        ];")
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.rules) != 2 {
		t.Fatalf("want items and tags once each, got %+v", rs.rules)
	}
	if it, _ := ruleByName(rs, "items"); it.required {
		t.Fatalf("items is not required itself (only its children are): %+v", it)
	}
}

func TestRulesNotLiteral(t *testing.T) {
	root := backWithRequest(t, "        return array_merge(parent::rules(), [\n            'a' => 'required',\n        ]);")
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if err != nil {
		t.Fatal(err)
	}
	if rs.literal {
		t.Fatalf("array_merge is not a literal list: %+v", rs)
	}
}

func TestRulesFromFormRequestUse(t *testing.T) {
	root := backWithRequest(t, "        return ['name' => 'required'];")
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if err != nil {
		t.Fatal(err)
	}
	if rs.source != "StoreOrderRequest" || rs.file != filepath.Join(root, "app/Modules/Sales/Orders/Requests/StoreOrderRequest.php") {
		t.Fatalf("got %+v", rs)
	}
	if n, ok := ruleByName(rs, "name"); !ok || !n.required {
		t.Fatalf("got %+v", rs.rules)
	}
}

func TestRulesInlineValidate(t *testing.T) {
	root := realDir(t)
	writeTree(t, root, map[string]string{orderCtl: "<?php\nclass OrderController\n{\n    public function other(Request $request)\n    {\n        $request->validate(['ghost' => 'required']);\n    }\n\n    public function store(Request $request)\n    {\n        $data = $request->validate([\n            'name' => 'required|string',\n            'notes' => ['nullable'],\n        ]);\n    }\n}\n"})
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if err != nil {
		t.Fatal(err)
	}
	if rs.source != "inline validate" || len(rs.rules) != 2 {
		t.Fatalf("want the two rules of store, not those of other: %+v", rs)
	}
	if n, _ := ruleByName(rs, "name"); !n.required || n.line != 12 {
		t.Fatalf("name = %+v", n)
	}
}

func TestRulesNone(t *testing.T) {
	root := backWithRequest(t, "        return [];")
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), "index")
	if err != nil {
		t.Fatal(err)
	}
	if rs.source != "" || len(rs.rules) != 0 {
		t.Fatalf("got %+v", rs)
	}
}

func TestRulesStayInsideRoot(t *testing.T) {
	root, outside := realDir(t), realDir(t)
	writeTree(t, outside, map[string]string{"Requests/StoreOrderRequest.php": "<?php\nclass StoreOrderRequest { public function rules() { return ['LEAKED' => 'required']; } }\n"})
	writeTree(t, root, map[string]string{orderCtl: "<?php\nuse App\\Linked\\Requests\\StoreOrderRequest;\nclass OrderController\n{\n    public function store(StoreOrderRequest $request) {}\n}\n"})
	if err := os.Symlink(outside, filepath.Join(root, "app/Linked")); err != nil {
		t.Fatal(err)
	}
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if err != nil {
		t.Fatal(err)
	}
	if rs.file != "" || len(rs.rules) != 0 || rs.literal {
		t.Fatalf("a request outside the backend must not be read, and its rules are unknown: %+v", rs)
	}
}
