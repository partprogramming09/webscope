package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mapSetup is a small Vue and Laravel tree with traps: a node_modules and a .env with secrets, and a symlinked folder that leads outside the root.
func mapSetup(t *testing.T) *coder {
	t.Helper()
	back, front, outside := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(base, rel, body string) {
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := "src/modules/crm/quotes/"
	write(front, m+"routes/QuotesRoutes.ts", "const QuotesRoutes = [\n    {\n        name: 'quotes',\n        path: '/crm/quotes',\n        component: () => import('@/modules/crm/quotes/views/QuoteIndex.vue'),\n    },\n    {\n        name: 'quotes-show',\n        path: '/crm/quotes/:quoteId',\n        component: () => import('@/modules/crm/quotes/views/QuoteShow.vue'),\n    },\n];\n")
	write(front, m+"views/QuoteIndex.vue", `<template>
  <div>
    <v-btn @click="openCreate" v-if="can('quotes.create')" title="Nueva">+</v-btn>
    <AppLink :to="'/crm/quotes/1'">Ver</AppLink>
    <v-text-field :label="$t('quotes.form.client')" required />
    <v-text-field :label="$t('quotes.form.notes')" />
    <v-btn v-if="can('quotes.delete')">x</v-btn>
  </div>
</template>
<script setup>
import { useQuotes } from '../composables/useQuotes'
</script>
`)
	write(front, m+"composables/useQuotes.ts", "export function useQuotes() {\n  api.get('/api/quotes?page=1')\n  api.get<Quote>(`/api/quotes/${id}`)\n  http.post(\"/api/quotes\", body)\n  api.put<{ ok: boolean }>(\n    `/api/quotes/${id}/lines`,\n    body\n  )\n}\n")
	write(front, m+"stores/quoteStore.ts", "export const useQuoteStore = defineStore('quote', {})\n")
	write(front, m+"constants/QuoteStatus.ts", "export const QuoteStatus = {\n  ENPROCESO: 'ENPROCESO',\n  APROBADA: 'APROBADA',\n}\n")
	write(front, "src/modules/crm/quotes/node_modules/dep/x.ts", "api.get('/api/SECRET_IN_NODE_MODULES')\n")
	write(front, ".env", "SECRET_IN_ENV=1\n")
	write(outside, "leak.ts", "api.get('/api/SECRET_OUTSIDE')\n")
	if err := os.Symlink(outside, filepath.Join(front, "src/modules/crm/quotes/linkdir")); err != nil {
		t.Fatal(err)
	}
	write(back, "app/Modules/Crm/Quotes/Routes/api.php", "<?php\nuse App\\Modules\\Crm\\Quotes\\Controllers\\QuoteController;\nuse Illuminate\\Support\\Facades\\Route;\n\nRoute::get('quotes', [QuoteController::class, 'index']);\nRoute::put('quotes/approve/{quote}', [QuoteController::class, 'approve']);\nRoute::apiResource('items', QuoteController::class)->only(['show']);\n")
	write(back, "app/Modules/Crm/Quotes/Controllers/QuoteController.php", "<?php\nnamespace App\\Modules\\Crm\\Quotes\\Controllers;\nuse App\\Modules\\Crm\\Quotes\\Services\\QuoteService;\nclass QuoteController {\n    public function index() {}\n    public function approve($quote) {}\n    public function show($id) {}\n}\n")
	write(back, "app/Modules/Crm/Quotes/Services/QuoteService.php", "<?php\nuse App\\Modules\\Crm\\Quotes\\Models\\Quote;\nclass QuoteService {}\n")
	write(back, "app/Modules/Crm/Quotes/Models/Quote.php", "<?php\nclass Quote { protected $table = 'app_cotizaciones'; }\n")
	write(back, ".env", "SECRET_IN_BACK_ENV=1\n")
	write(outside, "leak_ctrl.php", "<?php\nclass LeakController { public function index() {} } // SECRET_CTRL\n")
	if err := os.Symlink(filepath.Join(outside, "leak_ctrl.php"), filepath.Join(back, "app/Modules/Crm/Quotes/Controllers/LeakController.php")); err != nil {
		t.Fatal(err)
	}
	write(back, "app/Modules/Crm/Quotes/Routes/leak.php", "<?php\nuse App\\Modules\\Crm\\Quotes\\Controllers\\LeakController;\nRoute::get('leak', [LeakController::class, 'index']);\n")
	return &coder{s: &source{}, backend: back, front: front}
}

func TestMapListsRoutesCallsComposablesAndStores(t *testing.T) {
	out, err := code(t, mapSetup(t), "map", map[string]any{"module": "quotes"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## crm/quotes", "route /crm/quotes (quotes) -> @/modules/crm/quotes/views/QuoteIndex.vue", "routes/QuotesRoutes.ts:4",
		"route /crm/quotes/:quoteId (quotes-show)", "api GET /api/quotes?page=1", "composables/useQuotes.ts:2", "api GET /api/quotes/${id}", `api POST /api/quotes  `, "api PUT /api/quotes/${id}/lines  crm/quotes/composables/useQuotes.ts:5",
		"composable crm/quotes/composables/useQuotes.ts", "store crm/quotes/stores/quoteStore.ts"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if _, err := code(t, mapSetup(t), "map", map[string]any{}); err == nil {
		t.Error("map without a module must say what it needs")
	}
	if out, _ := code(t, mapSetup(t), "map", map[string]any{"module": "zzz"}); !strings.Contains(out, "no module contains «zzz»") {
		t.Errorf("an unknown module: %q", out)
	}
}

func TestMapWalkthroughGivesActionsPermissionsLinksFieldsAndStatuses(t *testing.T) {
	out, err := code(t, mapSetup(t), "map", map[string]any{"module": "quotes", "walkthrough": true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"### crm/quotes/views/QuoteIndex.vue [useQuotes: GET×2 POST PUT]", "action 3 openCreate if can('quotes.create') «Nueva»",
		"permission 7 v-if=can('quotes.delete')", "link 4 '/crm/quotes/1'", "fields 5 client*, notes", "statuses ENPROCESO APROBADA  crm/quotes/constants/QuoteStatus.ts"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "permission 3") {
		t.Errorf("the v-if of a button that has a click is the action's condition, not a separate permission:\n%s", out)
	}
}

func TestMapBackCrossesTheFrontWithRoutesControllerServiceModelAndTable(t *testing.T) {
	out, err := code(t, mapSetup(t), "map_back", map[string]any{"query": "quotes"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"routes read from: files Routes/*.php", "## front crm/quotes", "route /crm/quotes -> @/modules/crm/quotes/views/QuoteIndex.vue",
		"## backend", "api GET quotes -> QuoteController@index  app/Modules/Crm/Quotes/Controllers/QuoteController.php:5", "<- ", "useQuotes.ts:2",
		"api PUT quotes/approve/{} -> QuoteController@approve", "service app/Modules/Crm/Quotes/Services/QuoteService.php",
		"model app/Modules/Crm/Quotes/Models/Quote.php -> table app_cotizaciones"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if out, _ := code(t, mapSetup(t), "map_back", map[string]any{"query": "items"}); !strings.Contains(out, "api GET items/{} -> QuoteController@show") {
		t.Errorf("an apiResource with only(): %q", out)
	}
	if out, _ := code(t, mapSetup(t), "map_back", map[string]any{"query": "zzz"}); !strings.Contains(out, "no module or route contains «zzz»") {
		t.Errorf("an unknown query: %q", out)
	}
}

func TestMapNeverReadsNodeModulesEnvOrOutsideTheRoots(t *testing.T) {
	c := mapSetup(t)
	for _, a := range []struct {
		action string
		args   map[string]any
	}{{"map", map[string]any{"module": "quotes"}}, {"map", map[string]any{"module": "quotes", "walkthrough": true}}, {"map_back", map[string]any{"query": "quotes"}}, {"map", map[string]any{"module": ""}}} {
		out, _ := code(t, c, a.action, a.args)
		for _, secret := range []string{"SECRET_IN_NODE_MODULES", "SECRET_IN_ENV", "SECRET_OUTSIDE", "SECRET_IN_BACK_ENV", "linkdir", "leak.ts"} {
			if strings.Contains(out, secret) {
				t.Errorf("%s %v shows %q:\n%s", a.action, a.args, secret, out)
			}
		}
	}
}

func TestMapBackDoesNotOpenAControllerThatIsALinkOutOfTheRoot(t *testing.T) {
	out, err := code(t, mapSetup(t), "map_back", map[string]any{"query": "leak"})
	if err != nil || !strings.Contains(out, "api GET leak -> LeakController@index") || strings.Contains(out, "LeakController.php") || strings.Contains(out, "SECRET_CTRL") {
		t.Errorf("%q, %v", out, err)
	}
}

func TestMapNeedsItsFolderAndSaysWhich(t *testing.T) {
	c := mapSetup(t)
	c.front = ""
	if _, err := code(t, c, "map", map[string]any{"module": "quotes"}); err == nil || !strings.Contains(err.Error(), "WEBSCOPE_FRONT") {
		t.Errorf("map without WEBSCOPE_FRONT: %v", err)
	}
	c = mapSetup(t)
	c.backend = ""
	if _, err := code(t, c, "map_back", map[string]any{"query": "quotes"}); err == nil || !strings.Contains(err.Error(), "WEBSCOPE_BACKEND") {
		t.Errorf("map_back without WEBSCOPE_BACKEND: %v", err)
	}
}

func TestMapBackKeepsTheTablesWhenThereAreMoreEndpointsThanFit(t *testing.T) {
	c := mapSetup(t)
	var b strings.Builder
	b.WriteString("<?php\nuse App\\Modules\\Crm\\Quotes\\Controllers\\QuoteController;\n")
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "Route::get('quotes/extra-endpoint-number-%03d', [QuoteController::class, 'index']);\n", i)
	}
	if err := os.WriteFile(filepath.Join(c.backend, "app/Modules/Crm/Quotes/Routes/many.php"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := code(t, c, "map_back", map[string]any{"query": "quotes"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > maxBytes+200 || !strings.Contains(out, "model app/Modules/Crm/Quotes/Models/Quote.php -> table app_cotizaciones") || !strings.Contains(out, "more endpoints") {
		t.Errorf("%d bytes: the model and its table must survive and the cut must be said:\n...%s", len(out), out[max(0, len(out)-400):])
	}
	if !strings.Contains(out, "api GET quotes -> QuoteController@index") {
		t.Errorf("the first endpoints must still be there")
	}
}
