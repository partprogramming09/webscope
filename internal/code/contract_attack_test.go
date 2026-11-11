package code

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ---------- helpers ----------

// attackRules reads the rules of StoreOrderRequest::rules() with rulesBody as its body.
func attackRules(t *testing.T, rulesBody string) ruleSet {
	t.Helper()
	root := backWithRequest(t, rulesBody)
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

// attackCtl writes a controller with body ctl and reads the rules of method.
func attackCtl(t *testing.T, ctl string, extra map[string]string, method string) ruleSet {
	t.Helper()
	root := realDir(t)
	files := map[string]string{orderCtl: ctl}
	for k, v := range extra {
		files[k] = v
	}
	writeTree(t, root, files)
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), method)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func ruleNames(rs ruleSet) []string {
	var out []string
	for _, r := range rs.rules {
		out = append(out, r.name)
	}
	return out
}

// within runs f and fails if it does not end in d (a hang check).
func within(t *testing.T, d time.Duration, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("did not finish in %s (hang or exponential work)", d)
	}
}

const reqPath = "app/Modules/Sales/Orders/Requests/StoreOrderRequest.php"

// ---------- backend: failing ----------

// A block comment at the first level of the rules array must not create keys.
func TestAttackRulesBlockCommentCreatesKey(t *testing.T) {
	rs := attackRules(t, "        return [\n            /* 'ghost' => 'required', */\n            'a' => 'string',\n        ];")
	if want := []string{"a"}; !reflect.DeepEqual(ruleNames(rs), want) {
		t.Fatalf("got %v, want %v", ruleNames(rs), want)
	}
}

// An apostrophe inside a block comment must not swallow the next key.
func TestAttackRulesBlockCommentApostrophe(t *testing.T) {
	rs := attackRules(t, "        return [\n            /* don't touch */\n            'a' => 'required',\n            'b' => 'string',\n        ];")
	if want := []string{"a", "b"}; !reflect.DeepEqual(ruleNames(rs), want) {
		t.Fatalf("got %v, want %v", ruleNames(rs), want)
	}
}

// An apostrophe in a line comment inside a multi-line value must not break the parse.
func TestAttackRulesCommentApostropheInsideValue(t *testing.T) {
	rs := attackRules(t, "        return [\n            'a' => [ // don't change\n                'required',\n            ],\n            'b' => 'nullable',\n        ];")
	if want := []string{"a", "b"}; !reflect.DeepEqual(ruleNames(rs), want) {
		t.Fatalf("got %v, want %v", ruleNames(rs), want)
	}
	a, _ := ruleByName(rs, "a")
	b, _ := ruleByName(rs, "b")
	if !a.required || !b.nullable {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
}

// A comment after the last value (no trailing comma) is not part of the value.
func TestAttackRulesTrailingCommentNotRequired(t *testing.T) {
	rs := attackRules(t, "        return [\n            'a' => 'string' // will be required later\n        ];")
	a, ok := ruleByName(rs, "a")
	if !ok || a.required {
		t.Fatalf("a comment says 'required', the value does not: %+v", rs.rules)
	}
}

// A comment that mentions `return` before the real return must not hide the rules.
func TestAttackRulesCommentWithReturnWord(t *testing.T) {
	rs := attackRules(t, "        // return early when the form is a draft\n        return [\n            'a' => 'required',\n        ];")
	if want := []string{"a"}; !reflect.DeepEqual(ruleNames(rs), want) || !rs.literal {
		t.Fatalf("got %v literal=%v, want %v literal=true", ruleNames(rs), rs.literal, want)
	}
}

// A heredoc value with brackets and quotes must not break the parse.
func TestAttackRulesHeredocValue(t *testing.T) {
	rs := attackRules(t, "        return [\n            'a' => ['required', 'regex:' . <<<'RE'\nit's [a-z]\nRE],\n            'b' => 'nullable',\n        ];")
	if want := []string{"a", "b"}; !reflect.DeepEqual(ruleNames(rs), want) {
		t.Fatalf("got %v, want %v", ruleNames(rs), want)
	}
}

// With a plain Request first and the FormRequest second, the FormRequest is still the source.
func TestAttackRulesSecondParamIsFormRequest(t *testing.T) {
	ctl := "<?php\nnamespace App\\Modules\\Sales\\Orders\\Controllers;\n\nuse Illuminate\\Http\\Request;\nuse App\\Modules\\Sales\\Orders\\Requests\\StoreOrderRequest;\n\nclass OrderController\n{\n    public function store(Request $raw, StoreOrderRequest $request)\n    {\n    }\n}\n"
	req := "<?php\nclass StoreOrderRequest { public function rules() { return ['name' => 'required']; } }\n"
	rs := attackCtl(t, ctl, map[string]string{reqPath: req}, "store")
	if rs.source != "StoreOrderRequest" || !reflect.DeepEqual(ruleNames(rs), []string{"name"}) {
		t.Fatalf("got %+v", rs)
	}
}

// A commented-out validate([...]) in the method body must not be read instead of the real one.
func TestAttackRulesCommentedInlineValidate(t *testing.T) {
	ctl := "<?php\nclass OrderController\n{\n    public function store(Request $request)\n    {\n        // $request->validate(['old' => 'required']);\n        $request->validate(['name' => 'required']);\n    }\n}\n"
	rs := attackCtl(t, ctl, nil, "store")
	if !reflect.DeepEqual(ruleNames(rs), []string{"name"}) {
		t.Fatalf("got %v, want [name]", ruleNames(rs))
	}
}

// `function store(` mentioned in a comment of another method must not hide the real method.
func TestAttackRulesMethodNameInComment(t *testing.T) {
	ctl := "<?php\nclass OrderController\n{\n    public function create()\n    {\n        // the form posts to function store() below;\n        return view('x');\n    }\n\n    public function store(Request $request)\n    {\n        $request->validate(['name' => 'required']);\n    }\n}\n"
	rs := attackCtl(t, ctl, nil, "store")
	if !reflect.DeepEqual(ruleNames(rs), []string{"name"}) {
		t.Fatalf("got %+v, want rule name", rs)
	}
}

// ---------- backend: passing (pin edge cases) ----------

func TestAttackRulesRequiredLookalikes(t *testing.T) {
	rs := attackRules(t, "        return [\n            'a' => 'required_if:x,1',\n            'b' => ['required_with:y'],\n            'c' => [Rule::requiredIf(fn () => true)],\n            'd' => 'nullable_if_absent',\n            'e' => 'sometimes|required',\n            'f' => ['not_nullable'],\n        ];")
	for _, n := range []string{"a", "b", "c"} {
		if r, _ := ruleByName(rs, n); r.required {
			t.Errorf("%s must not be required: %+v", n, r)
		}
	}
	if r, _ := ruleByName(rs, "d"); r.nullable {
		t.Errorf("d must not be nullable: %+v", r)
	}
	if r, _ := ruleByName(rs, "f"); r.nullable {
		t.Errorf("f must not be nullable: %+v", r)
	}
	if r, _ := ruleByName(rs, "e"); r.required {
		t.Errorf("sometimes|required may be absent: %+v", r)
	}
}

func TestAttackRulesTrickyValuesNoSpuriousKeys(t *testing.T) {
	body := "        return [\n" +
		"            'a' => ['required', Rule::in(['x' => 1, 'y']), 'regex:/^[\\]\\[]+$/'],\n" +
		"            'b' => \"string|max:5\", # it's a comment\n" +
		"            'c' => [function ($attr, $value, $fail) {\n                if ($value === 'k => v') { $fail('bad ]'); }\n            }],\n" +
		"            'd' => 'in:\\'q\\',\"r\"',\n" +
		"            'e.*.x' => 'required',\n" +
		"            'e.0' => 'nullable',\n" +
		"            'f' => 'string',\n" +
		"        ];"
	rs := attackRules(t, body)
	if want := []string{"a", "b", "c", "d", "e", "f"}; !reflect.DeepEqual(ruleNames(rs), want) {
		t.Fatalf("got %v, want %v", ruleNames(rs), want)
	}
	if !rs.literal {
		t.Fatal("a plain [ … ]; is literal")
	}
	if f, _ := ruleByName(rs, "f"); f.line != 17 {
		t.Fatalf("f.line = %d, want 17", f.line)
	}
}

func TestAttackRulesPlusParentNotLiteral(t *testing.T) {
	rs := attackRules(t, "        return [\n            'a' => 'required',\n        ] + parent::rules();")
	if rs.literal {
		t.Fatalf("[...] + parent::rules() is not literal: %+v", rs)
	}
}

func TestAttackRulesUseAlias(t *testing.T) {
	ctl := "<?php\nnamespace App\\Modules\\Sales\\Orders\\Controllers;\n\nuse App\\Modules\\Sales\\Orders\\Requests\\StoreOrderRequest as SaveRequest;\nuse App\\Other\\SaveRequest as Unused;\n\nclass OrderController\n{\n    public function store(SaveRequest $request) {}\n}\n"
	req := "<?php\nclass StoreOrderRequest { public function rules() { return ['name' => 'required']; } }\n"
	rs := attackCtl(t, ctl, map[string]string{reqPath: req, "app/Other/SaveRequest.php": "<?php\nclass SaveRequest { public function rules() { return ['WRONG' => 'x']; } }\n"}, "store")
	if !reflect.DeepEqual(ruleNames(rs), []string{"name"}) {
		t.Fatalf("got %+v", rs)
	}
}

func TestAttackRulesNamespaceFallback(t *testing.T) {
	ctl := "<?php\nnamespace App\\Modules\\Sales\\Orders\\Requests;\n\nclass OrderController\n{\n    public function store(StoreOrderRequest $request) {}\n}\n"
	req := "<?php\nclass StoreOrderRequest { public function rules() { return ['name' => 'required']; } }\n"
	rs := attackCtl(t, ctl, map[string]string{reqPath: req}, "store")
	if !reflect.DeepEqual(ruleNames(rs), []string{"name"}) {
		t.Fatalf("got %+v", rs)
	}
}

// A request file that is itself a symlink to a file outside root must not be read.
func TestAttackRulesFileSymlinkOutside(t *testing.T) {
	root, outside := realDir(t), realDir(t)
	writeTree(t, outside, map[string]string{"x.php": "<?php\nclass StoreOrderRequest { public function rules() { return ['LEAKED' => 'required']; } }\n"})
	writeTree(t, root, map[string]string{orderCtl: "<?php\nuse App\\Requests\\StoreOrderRequest;\nclass OrderController\n{\n    public function store(StoreOrderRequest $request) {}\n}\n"})
	if err := os.MkdirAll(filepath.Join(root, "app/Requests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "x.php"), filepath.Join(root, "app/Requests/StoreOrderRequest.php")); err != nil {
		t.Fatal(err)
	}
	rs, err := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.rules) != 0 {
		t.Fatalf("read outside root: %+v", rs)
	}
}

func TestAttackRulesInlineValidateNotOtherMethod(t *testing.T) {
	ctl := "<?php\nclass OrderController\n{\n    public function store(Request $request)\n    {\n        return 1;\n    }\n\n    public function update(Request $request)\n    {\n        $request->validate(['ghost' => 'required']);\n    }\n}\n"
	rs := attackCtl(t, ctl, nil, "store")
	if len(rs.rules) != 0 || rs.source != "" {
		t.Fatalf("got %+v", rs)
	}
}

func TestAttackRulesAttributesAndAbstract(t *testing.T) {
	ctl := "<?php\nclass OrderController\n{\n    #[Route('/x;y', methods: ['POST'])]\n    public function store(#[CurrentUser] User $u, Request $request): JsonResponse\n    {\n        $request->validate([\n            'name' => 'required', // it's [\n            'notes' => 'nullable',\n        ]);\n    }\n}\n"
	rs := attackCtl(t, ctl, nil, "store")
	if want := []string{"name", "notes"}; !reflect.DeepEqual(ruleNames(rs), want) {
		t.Fatalf("got %v, want %v", ruleNames(rs), want)
	}
}

func TestAttackRulesGarbageNoHang(t *testing.T) {
	inputs := []string{"", "<?php function store(", "<?php function store() {", "<?php function store() { $r->validate([", "<?php function store() { $r->validate(['a' => ", "<?php function store() { $r->validate(['", "<?php function store(StoreOrderRequest $r) { /*", strings.Repeat("[", 10000), "<?php function store() { $r->validate([" + strings.Repeat("'a' => [", 5000)}
	for i, in := range inputs {
		root := realDir(t)
		writeTree(t, root, map[string]string{orderCtl: in})
		within(t, 5*time.Second, func() {
			if _, err := rulesFor(root, filepath.Join(root, orderCtl), "store"); err != nil {
				t.Errorf("input %d: %v", i, err)
			}
		})
	}
}

// ---------- front: failing ----------

// A line comment before a key (the usual `a: 1, // note` style) must not drop the next key.
func TestAttackSentCommentDropsKey(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save() {\n    /*CALL*/await axios.post('orders', {\n      id: 1, // the id\n      name: 'x',\n    })\n  }\n}\n", nil)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"id", "name"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("got %v (unresolved %v), want %v", names(got), unresolved, want)
	}
}

// A comparison in a value (`>` or `<`) must not drop the following keys.
func TestAttackSentComparisonDropsKey(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(items: number[], age: number) {\n    /*CALL*/await axios.post('orders', { hasItems: items.length > 0, adult: age >= 18, minor: age < 18, name: 'x' })\n  }\n}\n", nil)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"adult", "hasItems", "minor", "name"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("got %v (unresolved %v), want %v", names(got), unresolved, want)
	}
}

// A nested generic on the call (`post<Paged<Item>>(`) is still a write call.
func TestAttackSentNestedGeneric(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save() {\n    /*CALL*/await api.post<Paged<Item>>('orders', { a: 1 })\n  }\n}\n", nil)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"a"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("got %v (unresolved %v), want %v", names(got), unresolved, want)
	}
}

// In a union, a key that is optional (`?:`) in one member is optional in the union.
func TestAttackSentUnionOptionalInOneMember(t *testing.T) {
	types := map[string]string{"sales/orders/types/U.ts": "export type A = { x: string };\nexport type B = { x?: string };\n"}
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(info: A | B) {\n    /*CALL*/await axios.post('orders', info)\n  }\n}\n", types)
	got, _ := sentFields(base, file, line)
	if want := []string{"x?"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("got %v, want %v", names(got), want)
	}
}

// A line with no write call must not report the fields of a later call in the file.
func TestAttackSentLineWithoutCallTakesLaterCall(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  /*CALL*/const list = () => axios.get('orders')\n  const save = () => axios.post('orders', { ghost: 1 })\n}\n", nil)
	got, unresolved := sentFields(base, file, line)
	if len(got) != 0 {
		t.Fatalf("fields of another line: %v (unresolved %v)", names(got), unresolved)
	}
}

// Quoted keys with a dash are first-level fields too.
func TestAttackSentQuotedDashKey(t *testing.T) {
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save() {\n    /*CALL*/await axios.post('orders', { 'first-name': 'a', \"x-y\": 1 })\n  }\n}\n", nil)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"first-name", "x-y"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("got %v (unresolved %v), want %v", names(got), unresolved, want)
	}
}

// A type alias with an arrow-function member must not swallow the declarations below it.
func TestAttackSentTypeWithArrowMember(t *testing.T) {
	types := map[string]string{"sales/orders/types/T.ts": "export type Payload = { onDone: () => void } & Extra\nexport type Extra = { e: string }\n"}
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(info: Payload) {\n    /*CALL*/await axios.post('orders', info)\n  }\n}\n", types)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"e", "onDone"}; !reflect.DeepEqual(names(got), want) || len(unresolved) != 0 {
		t.Fatalf("got %v (unresolved %v), want %v", names(got), unresolved, want)
	}
}

// A diamond of type references must not take exponential time.
func TestAttackSentTypeDiamondNoHang(t *testing.T) {
	var b strings.Builder
	const n = 40
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "export type T%d = T%d & T%d\n", i, i+1, i+1)
	}
	fmt.Fprintf(&b, "export type T%d = { leaf: string }\n", n)
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(info: T0) {\n    /*CALL*/await axios.post('orders', info)\n  }\n}\n", map[string]string{"sales/orders/types/D.ts": b.String()})
	within(t, 5*time.Second, func() { sentFields(base, file, line) })
}

// package.json keys are case-sensitive for npm: "Dependencies" is not "dependencies".
func TestAttackCaseConverterKeyCase(t *testing.T) {
	d := realDir(t)
	writeTree(t, d, map[string]string{"package.json": `{"DEPENDENCIES": {"axios-case-converter": "1.1.1"}}`})
	if caseConverter(d) {
		t.Fatal("DEPENDENCIES is not dependencies for npm")
	}
}

// npm reads a package.json that starts with a UTF-8 BOM.
func TestAttackCaseConverterBOM(t *testing.T) {
	d := realDir(t)
	writeTree(t, d, map[string]string{"package.json": "\xef\xbb\xbf" + `{"dependencies": {"axios-case-converter": "1.1.1"}}`})
	if !caseConverter(d) {
		t.Fatal("a BOM at the start of package.json hides the dependency")
	}
}

// ---------- front: passing (pin edge cases) ----------

func TestAttackSentTrickyLiteralNoSpuriousKeys(t *testing.T) {
	body := "{\n" +
		"      a: 'x, y: {z}',\n" +
		"      b: () => ({ ghost: 1 }),\n" +
		"      c: (x) => x,\n" +
		"      save() { return { ghost2: 1 } },\n" +
		"      async load() {},\n" +
		"      d: { nested: 1, deep: { more: 2 } },\n" +
		"      'e': `t ${f(1, 2)} {`,\n" +
		"      \"g\": [1, 2],\n" +
		"      h,\n" +
		"      [computed]: 1,\n" +
		"    }"
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(h: number) {\n    /*CALL*/await http.patch<{ id: number }>('orders', "+body+")\n  }\n}\n", nil)
	got, _ := sentFields(base, file, line)
	if want := []string{"a", "b", "c", "d", "e", "g", "h"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("got %v, want %v", names(got), want)
	}
}

func TestAttackSentRecursiveTypesNoHang(t *testing.T) {
	types := map[string]string{"sales/orders/types/R.ts": "export type A = B & { a: string };\nexport type B = A & { b: string };\nexport interface C extends D { c: string }\nexport interface D extends C { d: string }\n"}
	for _, ty := range []string{"A", "C", "A | C"} {
		base, file, line := frontWith(t, "export function useOrders() {\n  async function save(info: "+ty+") {\n    /*CALL*/await axios.post('orders', info)\n  }\n}\n", types)
		within(t, 5*time.Second, func() { sentFields(base, file, line) })
	}
}

func TestAttackSentUnreadable(t *testing.T) {
	cases := map[string]string{
		"partial": "  async function save(info: Partial<Form>) {\n    /*CALL*/await axios.post('orders', info)\n  }\n",
		"call":    "  async function save(info: Form) {\n    /*CALL*/await axios.post('orders', build(info))\n  }\n",
		"notype":  "  async function save(info) {\n    /*CALL*/await axios.post('orders', info)\n  }\n",
		"array":   "  async function save(info: Form[]) {\n    /*CALL*/await axios.post('orders', info)\n  }\n",
		"newfd":   "  async function save() {\n    const fd = new FormData()\n    /*CALL*/await axios.post('orders', fd)\n  }\n",
	}
	for name, fn := range cases {
		base, file, line := frontWith(t, "export function useOrders() {\n"+fn+"}\n", formType)
		got, unresolved := sentFields(base, file, line)
		if len(got) != 0 || len(unresolved) != 1 {
			t.Errorf("%s: got %v unresolved %v; want no fields and one note", name, names(got), unresolved)
		}
	}
}

func TestAttackSentInterfaceExtendsMany(t *testing.T) {
	types := map[string]string{"sales/orders/types/I.ts": "export interface A { a: string; shared?: number }\nexport interface B { b?: string }\nexport interface C extends A, B {\n  c: number\n  shared: number\n  method(): void\n}\n"}
	base, file, line := frontWith(t, "export function useOrders() {\n  async function save(info: C) {\n    /*CALL*/await axios.post('orders', info)\n  }\n}\n", types)
	got, unresolved := sentFields(base, file, line)
	if want := []string{"a", "b?", "c", "shared"}; !reflect.DeepEqual(names(got), want) || len(unresolved) != 0 {
		t.Fatalf("got %v (unresolved %v), want %v", names(got), unresolved, want)
	}
}

func TestAttackSentNoBodyAndBadLine(t *testing.T) {
	base, file, _ := frontWith(t, "export function useOrders() {\n  /*CALL*/const go = () => axios.post('orders')\n}\n", nil)
	for _, l := range []int{-5, 0, 2, 999} {
		within(t, 2*time.Second, func() {
			got, _ := sentFields(base, file, l)
			if len(got) != 0 {
				t.Errorf("line %d: got %v", l, names(got))
			}
		})
	}
}

// snakeCase304 is a literal port of no-case@3 + snake-case@3.0.4 (split with \x00, strip, trim \x00, lower, join "_").
func snakeCase304(s string) string {
	for _, re := range splitWords {
		s = re.ReplaceAllString(s, "${1}\x00${2}")
	}
	s = notAlnum.ReplaceAllString(s, "\x00")
	s = strings.Trim(s, "\x00")
	parts := strings.Split(s, "\x00")
	for i := range parts {
		parts[i] = strings.ToLower(parts[i])
	}
	return strings.Join(parts, "_")
}

func TestAttackWireNameEdges(t *testing.T) {
	// expected values worked out by hand from snake-case@3.0.4 (no-case@3 split and strip regexps)
	for in, want := range map[string]string{"": "", "_": "", "__a__": "a", "a-b c.d": "a_b_c_d", "ABC": "abc", "ABCDef": "abc_def", "a1B2C": "a1_b2_c", "aBC": "a_bc", "x2": "x2", "2x": "2x", "IDs": "i_ds", "camelCaseID": "camel_case_id", "ñandú": "and", "foo__bar": "foo_bar", "$ref": "ref", "a_B": "a_b"} {
		if got := wireName(in, true); got != want {
			t.Errorf("wireName(%q) = %q, want %q", in, got, want)
		}
		if got := wireName(in, false); got != in {
			t.Errorf("wireName(%q, false) = %q", in, got)
		}
	}
}

func FuzzAttackWireName(f *testing.F) {
	for _, s := range []string{"quoteId", "URLPath", "a1B2C", "__x__", "ñ", "aBCDe"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if got, want := wireName(s, true), snakeCase304(s); got != want {
			t.Fatalf("wireName(%q) = %q, port says %q", s, got, want)
		}
	})
}

// ---------- fuzz: no panic ----------

func FuzzAttackRulesOfRequest(f *testing.F) {
	for _, s := range []string{"function rules() { return ['a' => 'required', 'b.*' => [x()]]; }", "function rules() { return [", "function rules() { return ['", "function rules() { return [/*", "function rules(){return['a'=>"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		rulesOfRequest(s)
		if _, body, at, ok := methodSpan(s, "store"); ok {
			if loc := inlineCall.FindStringIndex(body); loc != nil {
				open := at + loc[1] - 1
				if end := closing(s, open); end >= 0 {
					keysOf(s, open, end)
				}
			}
		}
	})
}

// FuzzAttackSentFields feeds s as the file with the call and as a .ts file of types; it only goes through sentFields (the real entry point).
func FuzzAttackSentFields(f *testing.F) {
	for _, s := range []string{"axios.post('u', { a: 1, ...b, c })", "api.put(u, x)", "http.patch<T>(u, {'a': 1, b() {}, c: () => 1})", "axios.post(u, {", "axios.post(u, {/*}*/})", "axios.post(u, ...{)", "type T = A & { a: string }\ntype A = T | { b?: x }", "interface T extends T { a: string }"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		dir := t.TempDir()
		call := filepath.Join(dir, "call.ts")
		typed := filepath.Join(dir, "typed.ts")
		if os.WriteFile(call, []byte(s), 0o644) != nil || os.WriteFile(typed, []byte("function f(info: T) {\n  axios.post(u, { ...info, x })\n}\n"), 0o644) != nil || os.WriteFile(filepath.Join(dir, "types.ts"), []byte(s), 0o644) != nil {
			t.Skip()
		}
		sentFields(dir, call, 1)
		sentFields(dir, typed, 2)
	})
}
