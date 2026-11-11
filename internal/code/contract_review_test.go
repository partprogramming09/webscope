package code

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSentSpreadCycleEnds(t *testing.T) {
	base, file, line := frontWith(t, "function toForm(payload: Api) {\n  const form = { ...payload }\n  return form\n}\nfunction save(form: Form) {\n  const payload = { ...form, extra: 1 }\n  /*CALL*/return axios.post('/api/orders', payload)\n}\n", nil)
	done := make(chan []string, 1)
	go func() { _, un := sentFields(base, file, line); done <- un }()
	select {
	case un := <-done:
		if len(un) == 0 {
			t.Fatal("a cycle of spreads must be named as incomplete")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a cycle of spreads does not end")
	}
}

func TestSentParamBeatsDeclarationOfAnotherFunction(t *testing.T) {
	base, file, line := frontWith(t, "function other() {\n  const data = { a: 1 }\n}\nfunction save(data: Form) {\n  /*CALL*/return axios.post('/api/orders', data)\n}\n", formType)
	got, _ := sentFields(base, file, line)
	if want := []string{"lines", "name", "notes?"}; !reflect.DeepEqual(names(got), want) {
		t.Fatalf("got %v, want %v", names(got), want)
	}
}

func TestSentDuplicateTypePrefersTheCallsModule(t *testing.T) {
	types := map[string]string{
		"aaa/types/Payload.ts":          "export interface Payload {\n  x: string\n}\n",
		"sales/orders/types/Payload.ts": "export interface Payload {\n  y: string\n}\n",
	}
	base, file, line := frontWith(t, "function save(info: Payload) {\n  /*CALL*/return axios.post('/api/orders', info)\n}\n", types)
	got, un := sentFields(base, file, line)
	if !reflect.DeepEqual(names(got), []string{"y"}) || len(un) != 0 {
		t.Fatalf("got %v, unresolved %v", names(got), un)
	}
}

func TestSentDuplicateTypeElsewhereIsAmbiguous(t *testing.T) {
	types := map[string]string{
		"aaa/types/Payload.ts": "export interface Payload {\n  x: string\n}\n",
		"bbb/types/Payload.ts": "export interface Payload {\n  y: string\n}\n",
	}
	base, file, line := frontWith(t, "function save(info: Payload) {\n  /*CALL*/return axios.post('/api/orders', info)\n}\n", types)
	got, un := sentFields(base, file, line)
	if len(got) != 0 || len(un) != 1 || !strings.Contains(un[0], "Payload") {
		t.Fatalf("two declarations equally far: got %v, unresolved %v", names(got), un)
	}
}

func TestSentFormDataWithoutType(t *testing.T) {
	base, file, line := frontWith(t, "function upload(file) {\n  const fd = new FormData()\n  fd.append('file', file)\n  /*CALL*/return axios.post('/api/orders/upload', fd)\n}\n", nil)
	_, un := sentFields(base, file, line)
	if len(un) != 1 || !strings.Contains(un[0], "FormData") {
		t.Fatalf("got %v", un)
	}
}

func TestRulesSometimesIsNotRequired(t *testing.T) {
	root := backWithRequest(t, "        return ['a' => 'sometimes|required|string', 'b' => ['sometimes', 'required']];")
	rs, _ := rulesFor(root, filepath.Join(root, orderCtl), "store")
	for _, r := range rs.rules {
		if r.required {
			t.Fatalf("sometimes|required may be absent: %+v", r)
		}
	}
}

func TestRulesThisValidateAndValidatorMake(t *testing.T) {
	for _, call := range []string{"$this->validate($request, [\n            'name' => 'required',\n        ]);", "$v = Validator::make($request->all(), [\n            'name' => 'required',\n        ]);"} {
		root := realDir(t)
		writeTree(t, root, map[string]string{orderCtl: "<?php\nclass OrderController\n{\n    public function store(Request $request)\n    {\n        " + call + "\n    }\n}\n"})
		rs, err := rulesFor(root, filepath.Join(root, orderCtl), "store")
		if err != nil || rs.source != "inline validate" || len(rs.rules) != 1 || !rs.rules[0].required {
			t.Fatalf("%s: got %+v, %v", call, rs, err)
		}
	}
}

func TestRulesRequestFileNotFoundIsNotNone(t *testing.T) {
	root := realDir(t)
	writeTree(t, root, map[string]string{orderCtl: "<?php\nuse Modules\\Sales\\Requests\\StoreOrderRequest;\nclass OrderController\n{\n    public function store(StoreOrderRequest $request) {}\n}\n"})
	rs, _ := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if rs.source != "StoreOrderRequest" || rs.file != "" || rs.literal {
		t.Fatalf("a FormRequest named but not found must say so: %+v", rs)
	}
}

func TestContractRequestFileNotFound(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;", true)
	writeTree(t, c.Backend, map[string]string{orderCtl: "<?php\nuse Modules\\Sales\\Requests\\StoreOrderRequest;\nclass OrderController\n{\n    public function store(StoreOrderRequest $request) {}\n}\n"})
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "POST orders"})
	if !strings.Contains(out, "rules StoreOrderRequest (file not found") || strings.Contains(out, "rules: none") || strings.Contains(out, "required, not sent") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestContractPatchPrefersPatchRoute(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;", true)
	writeTree(t, c.Backend, map[string]string{"app/Modules/Sales/Orders/Routes/api.php": "<?php\nuse App\\Modules\\Sales\\Orders\\Controllers\\OrderController;\nRoute::put('orders/{order}', [OrderController::class, 'update']);\nRoute::patch('orders/{order}', [OrderController::class, 'store']);\n"})
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "PATCH orders/1"})
	if !strings.Contains(out, "contract PATCH orders/{} -> OrderController@store") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestContractDropLabelIsConditional(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;\n  extraField: string;", true)
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "POST orders"})
	if !hasLine(out, "sent, not in rules (dropped if the method reads validated()): extra_field") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestContractTypesOutsideModules(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;", true)
	writeTree(t, c.Front, map[string]string{
		"src/modules/sales/orders/types/Order.ts":           "export type Other = { a: string };\n",
		"src/types/OrderForm.ts":                            "export type OrderForm = {\n  name: string;\n};\n",
		"src/modules/sales/orders/composables/useOrders.ts": "export function useOrders() {\n  async function create(customerId: string, info: OrderForm) {\n    return axios.post('orders', { ...info, customerId })\n  }\n}\n",
	})
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "POST orders"})
	if strings.Contains(out, "incomplete") || !strings.Contains(out, "sends 2") {
		t.Fatalf("a type under src/types must be found:\n%s", out)
	}
}

func TestContractIncompleteReadDoesNotAssertMissing(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;", true)
	writeTree(t, c.Front, map[string]string{"src/modules/sales/orders/composables/useOrders.ts": "export function useOrders() {\n  async function create(formData: FormData) {\n    return axios.post('orders', formData)\n  }\n}\n"})
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "POST orders"})
	if strings.Contains(out, "required, not sent") || !hasLine(out, "required, not in what could be read: customer_id, name, supplier_id") || !strings.Contains(out, "sends 0 (read incomplete)") {
		t.Fatalf("an incomplete read must not claim a field was not sent:\n%s", out)
	}
}

func TestContractGetHasNoBody(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;", true)
	writeTree(t, c.Backend, map[string]string{"app/Modules/Sales/Orders/Routes/api.php": "<?php\nuse App\\Modules\\Sales\\Orders\\Controllers\\OrderController;\nRoute::get('orders/{order}', [OrderController::class, 'store']);\n"})
	writeTree(t, c.Front, map[string]string{"src/modules/sales/orders/composables/useOrders.ts": "export function useOrders() {\n  async function show(id: string) {\n    return axios.get('orders/' + id)\n  }\n}\n"})
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "GET orders/3"})
	if !strings.Contains(out, "useOrders.ts:3 GET has no body: nothing compared") || strings.Contains(out, "required") && !strings.Contains(out, "keys,") {
		t.Fatalf("got:\n%s", out)
	}
	if strings.Contains(out, "not sent") || strings.Contains(out, "incomplete") {
		t.Fatalf("a GET compares nothing:\n%s", out)
	}
}

func TestSentVueReadsOnlyTheScript(t *testing.T) {
	base := realDir(t)
	writeTree(t, base, map[string]string{"sales/orders/views/Order.vue": "<template>\n  <p>Don't save twice</p>\n  <!-- it's a note -->\n</template>\n<script setup lang=\"ts\">\nasync function save() {\n  await axios.post('orders', { a: 1, b: 2 })\n}\n</script>\n"})
	got, un := sentFields(base, filepath.Join(base, "sales/orders/views/Order.vue"), 7)
	if !reflect.DeepEqual(names(got), []string{"a", "b"}) || len(un) != 0 {
		t.Fatalf("got %v, unresolved %v", names(got), un)
	}
}

func TestRulesInlineValidateWhenRequestClassIsNotFound(t *testing.T) {
	root := realDir(t)
	writeTree(t, root, map[string]string{orderCtl: "<?php\nuse Illuminate\\Http\\Request as HttpRequest;\nclass OrderController\n{\n    public function store(HttpRequest $request)\n    {\n        $request->validate(['name' => 'required']);\n    }\n}\n"})
	rs, _ := rulesFor(root, filepath.Join(root, orderCtl), "store")
	if rs.source != "inline validate" || len(rs.rules) != 1 {
		t.Fatalf("got %+v", rs)
	}
}

func TestContractLiteralCallFitsParamRoute(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;", true)
	writeTree(t, c.Backend, map[string]string{"app/Modules/Sales/Orders/Routes/api.php": "<?php\nuse App\\Modules\\Sales\\Orders\\Controllers\\OrderController;\nRoute::put('orders/{order}', [OrderController::class, 'update']);\nRoute::put('orders/bulk', [OrderController::class, 'bulk']);\n"})
	writeTree(t, c.Front, map[string]string{"src/modules/sales/orders/composables/useOrders.ts": "export function useOrders() {\n  async function a(info: OrderForm) {\n    return axios.put('orders/main', info)\n  }\n  async function b(info: OrderForm) {\n    return axios.put('orders/bulk', info)\n  }\n}\n"})
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "PUT orders/7"})
	if !strings.Contains(out, "useOrders.ts:3 sends") || strings.Contains(out, "useOrders.ts:6") {
		t.Fatalf("orders/main goes to orders/{order}; orders/bulk goes to its own route:\n%s", out)
	}
}
