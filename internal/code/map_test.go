package code

import (
	"fmt"
	"github.com/partprogramming09/webscope/internal/db"
	"github.com/partprogramming09/webscope/internal/input"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// mapSetup is a small Vue and Laravel tree with traps: a node_modules and a .env with secrets, and a symlinked folder that leads outside the root.
func mapSetup(t *testing.T) *Tool {
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
	write(front, m+"composables/useQuotes.ts", "export function useQuotes() {\n  api.get('/api/quotes?page=1')\n  api.get<Quote>(`/api/quotes/${id}`)\n  http.post(\"/api/quotes\", body)\n  api.put<{ ok: boolean }>(\n    `/api/quotes/${id}/lines`,\n    body\n  )\n  api.get('/api/quotes/' + id)\n}\n")
	write(front, m+"stores/quoteStore.ts", "export const useQuoteStore = defineStore('quote', {})\n")
	write(front, m+"constants/QuoteStatus.ts", "export const QuoteStatus = {\n  ENPROCESO: 'ENPROCESO',\n  APROBADA: 'APROBADA',\n}\n")
	write(front, "src/modules/crm/quotes/node_modules/dep/x.ts", "api.get('/api/SECRET_IN_NODE_MODULES')\n")
	write(front, ".env", "SECRET_IN_ENV=1\n")
	write(outside, "leak.ts", "api.get('/api/SECRET_OUTSIDE')\n")
	if err := os.Symlink(outside, filepath.Join(front, "src/modules/crm/quotes/linkdir")); err != nil {
		t.Fatal(err)
	}
	write(back, "app/Modules/Crm/Quotes/Routes/api.php", "<?php\nuse App\\Modules\\Crm\\Quotes\\Controllers\\QuoteController;\nuse Illuminate\\Support\\Facades\\Route;\n\nRoute::get('quotes', [QuoteController::class, 'index']);\nRoute::put('quotes/approve/{quote}', [QuoteController::class, 'approve']);\nRoute::get('quotes/{quote}', [QuoteController::class, 'show']);\nRoute::apiResource('items', QuoteController::class)->only(['show']);\n")
	write(back, "app/Modules/Crm/Quotes/Controllers/QuoteController.php", "<?php\nnamespace App\\Modules\\Crm\\Quotes\\Controllers;\nuse App\\Modules\\Crm\\Quotes\\Services\\QuoteService;\nuse App\\Linked\\Models\\Leak;\nclass QuoteController {\n    public function index() {}\n    public function approve($quote) {}\n    public function show($id) {}\n}\n")
	write(back, "app/Modules/Crm/Quotes/Services/QuoteService.php", "<?php\nuse App\\Modules\\Crm\\Quotes\\Models\\Quote;\nclass QuoteService {}\n")
	write(back, "app/Modules/Crm/Quotes/Models/Quote.php", "<?php\nclass Quote { protected $table = 'app_cotizaciones'; }\n")
	write(outside, "linked/Models/Leak.php", "<?php\nclass Leak { protected $table = 'OUTSIDE_VALUE'; }\n")
	if err := os.Symlink(filepath.Join(outside, "linked"), filepath.Join(back, "app/Linked")); err != nil { // a folder inside the root that leads out of it
		t.Fatal(err)
	}
	write(back, ".env", "SECRET_IN_BACK_ENV=1\n")
	write(outside, "leak_ctrl.php", "<?php\nclass LeakController { public function index() {} } // SECRET_CTRL\n")
	if err := os.Symlink(filepath.Join(outside, "leak_ctrl.php"), filepath.Join(back, "app/Modules/Crm/Quotes/Controllers/LeakController.php")); err != nil {
		t.Fatal(err)
	}
	write(back, "app/Modules/Crm/Quotes/Routes/leak.php", "<?php\nuse App\\Modules\\Crm\\Quotes\\Controllers\\LeakController;\nRoute::get('leak', [LeakController::class, 'index']);\n")
	return &Tool{Src: &db.Source{}, Backend: back, Front: front}
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
	for _, want := range []string{"### crm/quotes/views/QuoteIndex.vue [useQuotes: GET×3 POST PUT]", "action 3 openCreate if can('quotes.create') «Nueva»",
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
		"## backend", "api GET quotes -> QuoteController@index  app/Modules/Crm/Quotes/Controllers/QuoteController.php:6", "<- ", "useQuotes.ts:2",
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
	c.Front = ""
	if _, err := code(t, c, "map", map[string]any{"module": "quotes"}); err == nil || !strings.Contains(err.Error(), "WEBSCOPE_FRONT") {
		t.Errorf("map without WEBSCOPE_FRONT: %v", err)
	}
	c = mapSetup(t)
	c.Backend = ""
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
	if err := os.WriteFile(filepath.Join(c.Backend, "app/Modules/Crm/Quotes/Routes/many.php"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := code(t, c, "map_back", map[string]any{"query": "quotes"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > input.MaxAnswer+200 || !strings.Contains(out, "model app/Modules/Crm/Quotes/Models/Quote.php -> table app_cotizaciones") || !strings.Contains(out, "more endpoints") {
		t.Errorf("%d bytes: the model and its table must survive and the cut must be said:\n...%s", len(out), out[max(0, len(out)-400):])
	}
	if !strings.Contains(out, "api GET quotes -> QuoteController@index") {
		t.Errorf("the first endpoints must still be there")
	}
}

func TestMapBackNeverReadsAModelThroughALinkedFolderNorPanicsOnALongQuery(t *testing.T) {
	c := mapSetup(t)
	out, err := code(t, c, "map_back", map[string]any{"query": "quotes"})
	if err != nil || strings.Contains(out, "OUTSIDE_VALUE") || strings.Contains(out, "Leak.php") {
		t.Errorf("a model reached through a symbolic link to a folder outside the root: %q, %v", out, err)
	}
	long := strings.TrimSpace(strings.Repeat("quotes ", 1500))
	out, err = code(t, c, "map_back", map[string]any{"query": long})
	if err != nil || len(out) > input.MaxAnswer+300 {
		t.Errorf("a query of %d bytes: %d bytes back, %v", len(long), len(out), err)
	}
	if out, err = code(t, c, "map", map[string]any{"module": long}); err != nil || len(out) > input.MaxAnswer+300 {
		t.Errorf("map with a long module: %d bytes back, %v", len(out), err)
	}
}

func TestACallThatAddsAnIdAfterASlashIsTheShowEndpointNotTheList(t *testing.T) {
	out, err := code(t, mapSetup(t), "map_back", map[string]any{"query": "quotes"})
	if err != nil {
		t.Fatal(err)
	}
	var list, show string
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "api GET quotes -> QuoteController@index"):
			list = l
		case strings.HasPrefix(l, "api GET quotes/{} -> QuoteController@show"):
			show = l
		}
	}
	if list == "" || show == "" {
		t.Fatalf("the list and the show endpoints must be there:\n%s", out)
	}
	if strings.Contains(list, "useQuotes.ts:9") {
		t.Errorf("api.get('/api/quotes/' + id) is not the list call: %s", list)
	}
	if !strings.Contains(show, "useQuotes.ts:9") {
		t.Errorf("it is the call of the show endpoint: %s", show)
	}
}

func TestAMapBackThatFindsNothingSaysWhatToTry(t *testing.T) {
	out, err := code(t, mapSetup(t), "map_back", map[string]any{"query": "aprobar cotización"})
	if err != nil || !strings.Contains(out, "no module or route contains") || !strings.Contains(out, "words of an API address") || !strings.Contains(out, "in English") {
		t.Errorf("the answer to a query in Spanish must say that the code is in English and what a query can be: %q, %v", out, err)
	}
	out, err = code(t, mapSetup(t), "map", map[string]any{"module": "zzz"})
	if err != nil || !strings.Contains(out, "no module contains") || !strings.Contains(out, "crm/quotes") {
		t.Errorf("map must say what a module is: %q, %v", out, err)
	}
}

func TestFlowsJoinsTheScreenActionsWithTheirEndpointsInOneAnswer(t *testing.T) {
	out, err := code(t, mapSetup(t), "flows", map[string]any{"module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Flows of «crm/quotes»", "evidence, not user stories", "## screens", "route /crm/quotes (quotes)", "## endpoints",
		"api GET quotes -> QuoteController@index", "model app/Modules/Crm/Quotes/Models/Quote.php -> table app_cotizaciones"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	for _, secret := range []string{"SECRET_IN_ENV", "SECRET_IN_NODE_MODULES", "SECRET_OUTSIDE"} {
		if strings.Contains(out, secret) {
			t.Errorf("flows leaked %s", secret)
		}
	}
	if _, err := code(t, mapSetup(t), "flows", map[string]any{}); err == nil {
		t.Error("flows without a module must say what it needs")
	}
}

func TestFlowsWithoutABackendStillGivesTheScreensAndSaysSo(t *testing.T) {
	c := mapSetup(t)
	c.Backend = ""
	out, err := code(t, c, "flows", map[string]any{"module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## screens") || !strings.Contains(out, "WEBSCOPE_BACKEND is not set") {
		t.Errorf("want the screens and the missing backend named:\n%s", out)
	}
}

func TestCasesListsTheTestsEachFlowNeedsWithWhatRunsThemAndWhatChecksThem(t *testing.T) {
	out, err := code(t, mapSetup(t), "cases", map[string]any{"module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Cases of «crm/quotes»: a list to run, not a result", "id\tflow\tcase\trun with", "Verify: happy = page payloads",
		"QuoteIndex.vue\thappy: openCreate «Nueva»", "permission: quotes.create", "permission: quotes.delete",
		"required empty: client", "status: ENPROCESO, APROBADA", "PUT quotes/approve/{}\thttp error", "GET quotes\tempty", `simulate {match: "quotes/approve/", status: 500}`, " cases)"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	for _, not := range []string{"required empty: notes", "GET quotes/{}\tempty", "SECRET"} {
		if strings.Contains(out, not) {
			t.Errorf("must not contain %q:\n%s", not, out)
		}
	}
	if _, err := code(t, mapSetup(t), "cases", map[string]any{}); err == nil {
		t.Error("cases without a module must say what it needs")
	}
}

func TestCasesWithoutABackendStillGivesTheScreenCases(t *testing.T) {
	c := mapSetup(t)
	c.Backend = ""
	out, err := code(t, c, "cases", map[string]any{"module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "happy: openCreate") || strings.Contains(out, "\thttp error\t") {
		t.Errorf("want the screen cases and no endpoint cases:\n%s", out)
	}
}

func TestCasesKeepsEachCaseOnceAndDropsLabelsThatAreTranslationKeys(t *testing.T) {
	screens := "statuses A B  x.ts\n### crm/quotes/views/V.vue\naction 1 save «{{ $t('common.save') }}»\naction 2 remove «$t('x')»\naction 3 open «Abrir»\nfields 4 rate*, rate*, notes"
	out := casesFrom("crm/quotes", screens, "api DELETE quotes/{} -> C@d  f:1\napi GET quotes/{} -> C@s  f:2")
	if strings.Count(out, "required empty: rate") != 1 {
		t.Errorf("a case repeated:\n%s", out)
	}
	if strings.Contains(out, "$t(") || strings.Contains(out, "{{") {
		t.Errorf("a translation key is not a label a screen shows:\n%s", out)
	}
	for _, want := range []string{"happy: save\tclick the button that calls it", "happy: open «Abrir»\tclick «Abrir»", "DELETE quotes/{}\thttp error\tsimulate {match: \"quotes/\"", "crm/quotes\tstatus: A, B"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "GET quotes/{}\tempty") {
		t.Errorf("the show endpoint is not a list:\n%s", out)
	}
}

func TestFlowsKeepsRoomForTheEndpointsWhenTheScreensAreLong(t *testing.T) {
	c := mapSetup(t)
	big := "src/modules/crm/quotes/views/Big.vue"
	var b strings.Builder
	b.WriteString("<template><div>\n")
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&b, "<v-btn @click=\"doThing%d\" title=\"Boton %d\">x</v-btn>\n", i, i)
	}
	b.WriteString("</div></template>\n")
	p := filepath.Join(c.Front, big)
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := code(t, c, "flows", map[string]any{"module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## endpoints\nmodel") || !strings.Contains(out, "\napi GET quotes -> QuoteController@index") || !strings.Contains(out, "more lines of screens not shown") {
		t.Errorf("long screens must be cut and the endpoints must still be there:\n%.300s", out)
	}
}

func TestViewsComeBeforeComponentsAndStatusesStayOnTop(t *testing.T) {
	in := "## m\nroute /a\n### m/components/C.vue\naction 1 c\n### m/views/V.vue\naction 2 v\nstatuses A B  s.ts\n### m/views/W.vue\nfields 3 x*"
	out := viewsFirst(in)
	want := "## m\nroute /a\nstatuses A B  s.ts\n### m/views/V.vue\naction 2 v\n### m/views/W.vue\nfields 3 x*\n### m/components/C.vue\naction 1 c"
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestCasesGroupTheRequiredFieldsOfAScreenAndKeepBothPartsOfAPermission(t *testing.T) {
	out := casesFrom("m", "### m/views/F.vue\naction 1 save if can('INS', 'COTIZA')\nfields 2 a*, b, c*, a*", "")
	for _, want := range []string{"F.vue\trequired empty: a, c\tfill all but one of them at a time, save", "F.vue\tpermission: INS COTIZA\t"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "required empty") != 2 { // the legend and one row
		t.Errorf("the required fields of one screen are one row:\n%s", out)
	}
}

func manyEndpoints(t *testing.T, method string) *Tool {
	t.Helper()
	c := mapSetup(t)
	var b strings.Builder
	b.WriteString("<?php\nuse App\\Modules\\Crm\\Quotes\\Controllers\\QuoteController;\n")
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "Route::%s('quotes/extra-endpoint-number-%03d', [QuoteController::class, 'index']);\n", method, i)
	}
	if err := os.WriteFile(filepath.Join(c.Backend, "app/Modules/Crm/Quotes/Routes/many.php"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestFlowsKeepsTheTablesAndShortensTheEndpointLinesWhenThereAreManyEndpoints(t *testing.T) {
	out, err := code(t, manyEndpoints(t, "get"), "flows", map[string]any{"module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > input.MaxAnswer+200 || !strings.Contains(out, "-> table app_cotizaciones") {
		t.Errorf("%d bytes: the table must survive a long endpoint list", len(out))
	}
	if strings.Contains(out, "app/Modules/Crm/Quotes/Controllers/QuoteController.php") && strings.Contains(out, "api GET quotes -> QuoteController@index  app/") {
		t.Errorf("an endpoint line should carry the controller file name and line, not its whole path:\n%.600s", out)
	}
	if !strings.Contains(out, "api GET quotes -> QuoteController@index  QuoteController.php:6") {
		t.Errorf("want the short form `QuoteController.php:6`:\n%s", out)
	}
}

func TestCasesListsAnHttpErrorForEveryWriteEndpointEvenWhenThereAreHundreds(t *testing.T) {
	out, err := code(t, manyEndpoints(t, "post"), "cases", map[string]any{"module": "crm/quotes"})
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`\((\d+) cases, \d+ shown: ask for a narrower module\)`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("want the real count and the shown count:\n...%s", out[max(0, len(out)-200):])
	}
	if n, _ := strconv.Atoi(m[1]); n < 400 {
		t.Errorf("%d cases: every one of the 400 write endpoints needs its case", n)
	}
}
