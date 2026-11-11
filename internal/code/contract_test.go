package code

import (
	"strings"
	"testing"

	"github.com/partprogramming09/webscope/internal/db"
)

// contractSetup is a Laravel and Vue pair: POST orders validates StoreOrderRequest, PUT orders/{order} validates inline, and the composable sends
// { ...info, customerId } on create and info on update; storeRules is the body of StoreOrderRequest::rules.
func contractSetup(t *testing.T, storeRules, formType string, converter bool) *Tool {
	t.Helper()
	back, front := realDir(t), realDir(t)
	writeTree(t, back, map[string]string{
		"app/Modules/Sales/Orders/Routes/api.php": "<?php\nuse App\\Modules\\Sales\\Orders\\Controllers\\OrderController;\nuse Illuminate\\Support\\Facades\\Route;\n\nRoute::post('orders', [OrderController::class, 'store']);\nRoute::put('orders/{order}', [OrderController::class, 'update']);\nRoute::post('orders/bulk', [OrderController::class, 'bulk']);\n",
		orderCtl: "<?php\nnamespace App\\Modules\\Sales\\Orders\\Controllers;\n\nuse App\\Modules\\Sales\\Orders\\Requests\\StoreOrderRequest;\n\nclass OrderController\n{\n    public function store(StoreOrderRequest $request) {}\n\n    public function update(Request $request, $order)\n    {\n        $request->validate(['name' => 'required']);\n    }\n\n    public function bulk(Request $request) {}\n}\n",
		"app/Modules/Sales/Orders/Requests/StoreOrderRequest.php": "<?php\nnamespace App\\Modules\\Sales\\Orders\\Requests;\n\nclass StoreOrderRequest extends FormRequest\n{\n    public function rules(): array\n    {\n        return [\n" + storeRules + "\n        ];\n    }\n}\n",
	})
	pkg := `{"dependencies": {"axios": "1"}}`
	if converter {
		pkg = `{"dependencies": {"axios": "1", "axios-case-converter": "1.1.1"}}`
	}
	writeTree(t, front, map[string]string{
		"package.json": pkg,
		"src/modules/sales/orders/composables/useOrders.ts": "export function useOrders() {\n  async function create(customerId: string, info: OrderForm) {\n    return axios.post('orders', { ...info, customerId })\n  }\n  async function update(id: string, info: OrderForm) {\n    return axios.put('orders/' + id, info)\n  }\n}\n",
		"src/modules/sales/orders/types/Order.ts":           "export type OrderForm = {\n" + formType + "\n};\n",
	})
	return &Tool{Src: &db.Source{}, Backend: back, Front: front}
}

const orderRules = "            'customer_id' => ['required'],\n            'name' => 'required|string',\n            'supplier_id' => ['required'],\n            'notes' => ['nullable'],"

func TestContractReportsGaps(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;\n  notes?: string | null;\n  extraField: string;", true)
	out, err := code(t, c, "contract", map[string]any{"endpoint": "POST /api/orders"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"contract POST orders -> OrderController@store  app/Modules/Sales/Orders/Controllers/OrderController.php:8",
		"rules StoreOrderRequest  app/Modules/Sales/Orders/Requests/StoreOrderRequest.php: 4 keys, 3 required",
		"front src/modules/sales/orders/composables/useOrders.ts:3 sends 4 (camelCase -> snake_case: axios-case-converter)",
		"sent, not in rules (dropped if the method reads validated()): extra_field",
		"required, not sent: supplier_id",
	} {
		if !hasLine(out, want) && !strings.Contains(out, want+"  ") {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "near names") || strings.Contains(out, "incomplete") {
		t.Errorf("nothing else expected:\n%s", out)
	}
}

func TestContractNearNames(t *testing.T) {
	c := contractSetup(t, "            'customer_id' => ['required'],\n            'shipping_address' => ['required'],", "  shippingAdress: string;", true)
	out, err := code(t, c, "contract", map[string]any{"endpoint": "POST orders"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "near names: sent shipping_adress ~ rule shipping_address") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestContractNearNamesWithoutId(t *testing.T) {
	c := contractSetup(t, "            'customer_id' => ['required'],\n            'warehouse_id' => ['required'],", "  warehouse: string;", true)
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "POST orders"})
	if !strings.Contains(out, "near names: sent warehouse ~ rule warehouse_id") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestContractOptionalVsRequired(t *testing.T) {
	c := contractSetup(t, orderRules, "  name?: string;\n  supplierId: string;\n  notes?: string;", true)
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "POST orders"})
	if !hasLine(out, "required in rules, optional in the front type: name") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestContractParamPath(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;\n  extra: string;", true)
	out, err := code(t, c, "contract", map[string]any{"endpoint": "PUT /api/orders/42"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"contract PUT orders/{} -> OrderController@update", "rules inline validate", "useOrders.ts:6 sends 2", "sent, not in rules (dropped if the method reads validated()): extra"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestContractNoRules(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;", true)
	out, err := code(t, c, "contract", map[string]any{"endpoint": "POST orders/bulk"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "rules: none (no FormRequest or validate in the method)") || !strings.Contains(out, "front: no call to this endpoint in WEBSCOPE_FRONT") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestContractWithoutConverter(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;\n  supplier_id: string;", false)
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "POST orders"})
	if !strings.Contains(out, "(names compared as written)") || !strings.Contains(out, "sent, not in rules (dropped if the method reads validated()): customerId") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestContractIncompleteType(t *testing.T) {
	c := contractSetup(t, orderRules, "  name: string;", true)
	writeTree(t, c.Front, map[string]string{"src/modules/sales/orders/types/Order.ts": "export type Other = { a: string };\n"})
	out, _ := code(t, c, "contract", map[string]any{"endpoint": "POST orders"})
	if !strings.Contains(out, "incomplete: ...info (type OrderForm not found") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestContractUnknownEndpoint(t *testing.T) {
	_, err := code(t, contractSetup(t, orderRules, "", true), "contract", map[string]any{"endpoint": "POST zzz"})
	if err == nil || !strings.Contains(err.Error(), "no route POST zzz") || !strings.Contains(err.Error(), "map_back") {
		t.Fatalf("got %v", err)
	}
}

func TestContractNeedsMethod(t *testing.T) {
	_, err := code(t, contractSetup(t, orderRules, "", true), "contract", map[string]any{"endpoint": "orders"})
	if err == nil || !strings.Contains(err.Error(), "METHOD /path") {
		t.Fatalf("got %v", err)
	}
}

// hasLine reports whether out has a line that is exactly want, or starts with want and a space.
func hasLine(out, want string) bool {
	for _, l := range strings.Split(out, "\n") {
		if l == want || strings.HasPrefix(l, want+" ") {
			return true
		}
	}
	return false
}
