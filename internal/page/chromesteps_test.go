package page

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// runSteps loads xhrPage from a test app in a real Chrome and runs each step of JavaScript in it, in order; step 0 is the snippet of page(load).
// extra serves other paths of the app. It returns each step's result and the Tool.
func runSteps(t *testing.T, steps func(load string) []string, extra func(w http.ResponseWriter, r *http.Request) bool) ([]string, *Tool) {
	t.Helper()
	return runStepsTool(t, func(_ *Tool, load string) []string { return steps(load) }, extra)
}

// runStepsTool is runSteps with the Tool given to steps, for a step that asks the Tool for a snippet.
func runStepsTool(t *testing.T, steps func(p *Tool, load string) []string, extra func(w http.ResponseWriter, r *http.Request) bool) ([]string, *Tool) {
	t.Helper()
	var mu sync.Mutex
	var html_ string
	app := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if extra != nil && extra(w, r) {
			return
		}
		io.WriteString(w, html_)
	}))
	app.Start()
	t.Cleanup(app.Close)
	p := testPage(t, app.URL)
	load, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	list := steps(p, load)
	raw, _ := json.Marshal(list)
	mu.Lock()
	html_ = strings.Replace(xhrPage, "{{STEPS}}", string(raw), 1)
	mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	flags := []string{"--headless=new", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--user-data-dir=" + t.TempDir(), "--virtual-time-budget=60000", "--dump-dom"}
	if os.Getenv("CI") != "" {
		flags = append(flags, "--no-sandbox")
	}
	dom, err := exec.CommandContext(ctx, chromePath(t), append(flags, app.URL+"/")...).Output()
	if err != nil {
		t.Fatalf("chrome: %v", err)
	}
	m := regexp.MustCompile(`(?s)<pre id="out">(.*?)</pre>`).FindSubmatch(dom)
	if m == nil {
		t.Fatalf("the page did not finish its steps; DOM tail: %.600s", dom[max(0, len(dom)-600):])
	}
	var out []string
	if err := json.Unmarshal([]byte(html.UnescapeString(string(m[1]))), &out); err != nil || len(out) != len(list) {
		t.Fatalf("results %q: %v", m[1], err)
	}
	return out, p
}

func TestStateShowsQueryParameterNamesNeverValuesAndSaysWhenTheScreenIsA404(t *testing.T) {
	out, _ := runSteps(t, func(load string) []string {
		return []string{
			load,
			"(history.replaceState(null,'','/p?token=SECRET9&id=7#top'),'ok')",
			"window.__ws.state({})",
			"(document.title='404 Not Found',document.body.insertAdjacentHTML('beforeend','<h1>Page not found</h1>'),'ok')",
			"window.__ws.state({})",
		}
	}, nil)
	if strings.Contains(out[2], "SECRET9") || strings.Contains(out[2], "token=") || !strings.Contains(out[2], "URL: /p ?token,id") {
		t.Errorf("the URL line must carry parameter names, not values: %q", out[2])
	}
	if strings.HasPrefix(out[2], "Looks like a 404") {
		t.Errorf("a normal screen was called a 404: %q", out[2])
	}
	if !strings.HasPrefix(out[4], "Looks like a 404") {
		t.Errorf("a 404 screen must say so on the first line: %q", out[4])
	}
}

func TestStateOverTheLimitDropsWholeLinesInsteadOfCuttingAString(t *testing.T) {
	out, _ := runSteps(t, func(load string) []string {
		return []string{
			load,
			"(document.title='A title that is long enough to be cut in the middle if the limit were applied to characters',document.body.insertAdjacentHTML('beforeend','<h1>Heading one</h1><h2>Heading two</h2>'),'ok')",
			"window.__ws.state({max:60})",
		}
	}, nil)
	if strings.Contains(out[2], "…(+") || !strings.Contains(out[2], "more lines not shown") {
		t.Errorf("state cut a string or did not say how many lines went: %q", out[2])
	}
}

func TestReactNativeWebPressablesAreButtonsForStateFindAndAudit(t *testing.T) {
	out, _ := runSteps(t, func(load string) []string {
		return []string{
			load,
			"(document.body.insertAdjacentHTML('beforeend','<div tabindex=0 style=\"cursor:pointer\"><div dir=auto>Soy importador</div></div><div tabindex=0 style=\"cursor:pointer;width:20px;height:20px\"></div><div tabindex=0 style=\"overflow:auto\">Region con scroll</div>'),'ok')",
			"window.__ws.state({})",
			"window.__ws.find({regex:'importador'})",
			"window.__ws.audit({})",
		}
	}, nil)
	if !strings.Contains(out[2], "Soy importador") {
		t.Errorf("state must list the pressable as a button: %q", out[2])
	}
	if !strings.Contains(out[3], "[button] Soy importador") {
		t.Errorf("find must return the pressable as a button: %q", out[3])
	}
	if strings.Contains(out[2], "Region con scroll") || strings.Contains(out[3], "Region con scroll") {
		t.Errorf("a focusable region with no pointer cursor is not a button: %q %q", out[2], out[3])
	}
	if !strings.Contains(out[4], "no name") {
		t.Errorf("audit must report the pressable with no text as an unnamed button: %q", out[4])
	}
}

func TestAFailedRequestBringsOnlyTheErrorLinesWrittenSinceTheClick(t *testing.T) {
	logPath := t.TempDir() + "/php.log"
	os.WriteFile(logPath, []byte("[10:00:00] PHP Fatal error: OLD error from before the click\n[10:00:02] GET /api/ok 200\n"), 0o600)
	t.Setenv("WEBSCOPE_LOG", logPath)
	app := func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/api/boom":
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o600)
			f.WriteString("[10:05:00] GET /api/ok 200\n[10:05:01] PHP Fatal error: NEW error caused by the click\n")
			f.Close()
			http.Error(w, "no", http.StatusInternalServerError)
			return true
		case "/api/ok":
			io.WriteString(w, "{}")
			return true
		}
		return false
	}
	out, p := runSteps(t, func(load string) []string {
		return []string{
			load,
			`(document.body.insertAdjacentHTML('beforeend','<button onclick="fetch(\'/api/ok\')">Fine</button><button onclick="fetch(\'/api/boom\')">Boom</button>'),'ok')`,
			"window.__ws.click({text:'Fine'})",
			"window.__ws.click({text:'Boom'})",
		}
	}, app)
	if strings.Contains(out[2], "Backend log") {
		t.Errorf("a request that went well brought the log: %q", out[2])
	}
	if !strings.Contains(out[3], "NEW error caused by the click") || strings.Contains(out[3], "OLD error") || strings.Contains(out[3], "GET /api/ok 200") {
		t.Errorf("a 500 must bring only the error lines written since the click: %q", out[3])
	}
	if seen, _ := askPage(t, p, "payloads", nil); !strings.Contains(seen, "NEW error caused by the click") || strings.Contains(seen, "OLD error") {
		t.Errorf("payloads must bring only the lines since the last click: %s", seen)
	}
}

func TestTheLogRouteNeedsTheTokenAndAnswersNothingWithoutAConfiguredLog(t *testing.T) {
	t.Setenv("WEBSCOPE_LOG", "")
	p := testPage(t, appOrigin)
	if got := p.logLines(); got != "" {
		t.Errorf("no log configured must give nothing: %q", got)
	}
}

func TestClickSaysWhenARequestWasStillPendingInsteadOfNoEffect(t *testing.T) {
	app := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/ok" {
			io.WriteString(w, "{}")
			return true
		}
		return false
	}
	// virtual time waits for the network, so a slow request cannot stay pending in this Chrome: one request is left counted by the helper
	// (its send reaches the wrapper and then fails on a stand-in object) and never ends.
	stuck := `(()=>{const P=Object.getPrototypeOf(new XMLHttpRequest());try{P.send.call({__ws:{m:'GET',u:'/stuck'},addEventListener(){}},null)}catch(e){}return 'stuck'})()`
	out, _ := runSteps(t, func(load string) []string {
		return []string{
			load,
			`(document.body.insertAdjacentHTML('beforeend','<button onclick="fetch(\'/api/ok\')">Fine</button><button>Noop</button>'),'ok')`,
			"window.__ws.click({text:'Fine'})",
			stuck,
			"window.__ws.click({text:'Noop'})",
		}
	}, app)
	if strings.Contains(out[2], "pending") {
		t.Errorf("a request that answered in time was called pending: %q", out[2])
	}
	if !strings.Contains(out[4], "Still pending") || strings.Contains(out[4], "No effect seen") {
		t.Errorf("a request still pending must be said, not «No effect seen»: %q", out[4])
	}
}

func TestClickShowsTheUrlChangeWithParameterNamesNeverValues(t *testing.T) {
	out, _ := runSteps(t, func(load string) []string {
		return []string{
			load,
			`(document.body.insertAdjacentHTML('beforeend','<button onclick="history.pushState(null,\'\',\'/next?token=SECRET9&id=7#top\')">Go</button>'),'ok')`,
			"window.__ws.click({text:'Go'})",
		}
	}, nil)
	if strings.Contains(out[2], "SECRET9") || strings.Contains(out[2], "token=") || !strings.Contains(out[2], "/next ?token,id") {
		t.Errorf("the URL change must carry parameter names, not values: %q", out[2])
	}
}

func TestAnOpaqueNoCorsAnswerIsNotCountedAsAFailure(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "{}") }))
	defer other.Close()
	out, p := runSteps(t, func(load string) []string {
		return []string{
			load,
			`(document.body.insertAdjacentHTML('beforeend','<button onclick="fetch(\'` + other.URL + `/x\',{mode:\'no-cors\'})">Opaque</button>'),'ok')`,
			"window.__ws.click({text:'Opaque'})",
		}
	}, nil)
	if strings.Contains(out[2], "no answer") || !strings.Contains(out[2], "opaque") {
		t.Errorf("click must say opaque, not no answer: %q", out[2])
	}
	seen, _ := askPage(t, p, "payloads", nil)
	if strings.Contains(seen, "Problems:") || !strings.Contains(seen, "opaque") {
		t.Errorf("payloads must not count an opaque answer as a problem, and must say it is opaque: %s", seen)
	}
}

func TestAnOpaqueRedirectAnswerIsNotCountedAsAFailureEither(t *testing.T) {
	app := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/go" {
			http.Redirect(w, r, "/api/ok", http.StatusFound)
			return true
		}
		return false
	}
	out, p := runSteps(t, func(load string) []string {
		return []string{
			load,
			`(document.body.insertAdjacentHTML('beforeend','<button onclick="fetch(\'/go\',{redirect:\'manual\'})">Redirect</button>'),'ok')`,
			"window.__ws.click({text:'Redirect'})",
		}
	}, app)
	if strings.Contains(out[2], "no answer") || !strings.Contains(out[2], "opaque") {
		t.Errorf("click must say opaque, not no answer: %q", out[2])
	}
	if seen, _ := askPage(t, p, "payloads", nil); strings.Contains(seen, "Problems:") {
		t.Errorf("an opaque redirect is not a problem: %s", seen)
	}
}

func TestAnActionLoadsTheHelperItselfWhenThePageLostIt(t *testing.T) {
	out, _ := runStepsTool(t, func(p *Tool, _ string) []string {
		state, err := askPage(t, p, "state", nil)
		if err != nil {
			t.Fatal(err)
		}
		return []string{
			"String(typeof window.__ws)",
			state,
			"String(typeof window.__ws)",
		}
	}, nil)
	if out[0] != "undefined" {
		t.Fatalf("the page must start without the helper: %q", out[0])
	}
	if !strings.Contains(out[1], "URL:") || strings.Contains(out[1], "Failed") {
		t.Errorf("state on a page without the helper must load it and answer: %q", out[1])
	}
	if out[2] != "object" {
		t.Errorf("the helper must stay loaded after the action: %q", out[2])
	}
}

func TestAFetchWrapperThatCallsTheHelperIsANoteNotAnInterceptor(t *testing.T) {
	out, p := runSteps(t, func(load string) []string {
		return []string{
			load,
			"(()=>{const f=window.fetch;window.fetch=(...a)=>f(...a);return 'wrapped'})()",
			"window.__ws.state({})",
			"new Promise(r=>setTimeout(()=>r('waited'),1200))",
		}
	}, nil)
	if strings.Contains(out[2], "Interceptor:") || !strings.HasPrefix(out[2], "Note: fetch was wrapped") {
		t.Errorf("a wrapper that calls the helper must give a note, not the interceptor warning: %q", out[2])
	}
	seen, _ := askPage(t, p, "payloads", nil)
	if strings.Contains(seen, "Interceptor:") || !strings.Contains(seen, "Note: fetch was wrapped") {
		t.Errorf("payloads must carry the note and not the warning: %s", seen)
	}
	if p.tamperedEver {
		t.Errorf("a wrapper that calls the helper must not downgrade the evidence")
	}
}

func TestARequestToAnApiOnAnotherOriginIsCapturedWithOnlyTheTabListed(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(api.Close)
	_, p := runSteps(t, func(load string) []string {
		return []string{load, "fetch('" + api.URL + "/api/items').then(r=>r.text())", "new Promise(r=>setTimeout(()=>r('waited'),600))"}
	}, nil)
	out, _ := askPage(t, p, "payloads", nil)
	if !strings.Contains(out, "/api/items") || strings.Contains(out, "refused") {
		t.Errorf("a request to an API on another origin must be captured and not refused: %s", out)
	}
}

func TestTheRefusalSaysThatOnlyTheTabOriginCountsAndNeverRepeatsIt(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	send(t, "POST", srv.URL+"/capture?t="+p.token, "http://demo.localhost:5173", `{"m":"GET","u":"/x","s":200}`)
	out, _ := askPage(t, p, "payloads", nil)
	if !strings.Contains(out, "Only the tab's origin counts") || strings.Contains(out, "demo.localhost") {
		t.Errorf("the refusal must say only the tab's origin counts and not repeat the origin: %s", out)
	}
}

func TestStateCutByMaxKeepsTheDialogAndItsFieldsAndSaysMaxCountsCharacters(t *testing.T) {
	out, _ := runSteps(t, func(load string) []string {
		return []string{
			load,
			"(document.body.insertAdjacentHTML('beforeend',Array.from({length:60},(_,i)=>'<label>Filtro '+i+'<input></label>').join('')+'<div role=dialog><p>Nuevo usuario</p><label>Correo<input></label><button>Guardar</button></div>'),'ok')",
			"window.__ws.state({max:600})",
		}
	}, nil)
	for _, want := range []string{"Dialog: Nuevo usuario", "Guardar", "[text] Correo", "max counts characters (600 now)"} {
		if !strings.Contains(out[2], want) {
			t.Errorf("state cut by max lost %q: %q", want, out[2])
		}
	}
	if strings.Contains(out[2], "Filtro 59") {
		t.Errorf("the page's long list must be what the cut drops: %q", out[2])
	}
}

func TestStateCutByMaxSkipsALongLineInsteadOfDroppingEverythingAfterIt(t *testing.T) {
	out, _ := runSteps(t, func(load string) []string {
		return []string{
			load,
			"(document.body.insertAdjacentHTML('beforeend','<div role=dialog><p>Nuevo usuario</p><button>Guardar</button><label>Correo<input></label></div>'),console.error('x'.repeat(190)),console.error('y'.repeat(190)),'ok')",
			"new Promise(r=>setTimeout(()=>r(window.__ws.state({max:420})),300))",
		}
	}, nil)
	for _, want := range []string{"Dialog: Nuevo usuario", "Guardar", "[text] Correo"} {
		if !strings.Contains(out[2], want) {
			t.Errorf("a long line before it hid %q: %q", want, out[2])
		}
	}
}

func TestFillSaysWhetherTheSaveButtonIsDisabledAndWhichFieldsShowAnError(t *testing.T) {
	out, _ := runSteps(t, func(load string) []string {
		return []string{
			load,
			`(document.body.insertAdjacentHTML('beforeend','<div role=dialog><div class="v-input v-input--error"><label for=mail>Correo</label><input id=mail><div class=v-messages__message>Correo no válido</div></div><label for=nom>Nombre</label><input id=nom><button id=sv disabled>Guardar</button></div>'),document.getElementById('mail').addEventListener('input',e=>{document.getElementById('sv').disabled=!e.target.value.includes('@')}),'ok')`,
			`window.__ws.fill({fields:{Correo:"x"}})`,
			`window.__ws.fill({fields:{Correo:"a@b.co"}})`,
		}
	}, nil)
	if !strings.Contains(out[2], "«Guardar» is disabled; fields with an error: Correo (Correo no válido).") {
		t.Errorf("an invalid form must say the button is disabled and name the field: %q", out[2])
	}
	if !strings.Contains(out[3], "«Guardar» is enabled") {
		t.Errorf("a valid form must say the button is enabled: %q", out[3])
	}
}

func TestTheHelperTellsTheListenerWhatKindOfAnswerEachActionGave(t *testing.T) {
	var buf syncBuf
	runStepsTool(t, func(p *Tool, load string) []string {
		p.Trace = &buf
		return []string{load, `window.__ws.click({text:"ZZ secret button"})`, `window.__ws.state({})`, `new Promise(r=>setTimeout(()=>r('waited'),700))`}
	}, nil)
	got := buf.String()
	for _, want := range []string{"webscope: event page click failed", "webscope: event page click nothing_to_click", "webscope: event page state ok"} {
		if !strings.Contains(got, want) {
			t.Errorf("the log lacks %q: %q", want, got)
		}
	}
	if strings.Contains(got, "secret") {
		t.Errorf("no text of the call may reach the log: %q", got)
	}
}

func TestTheEventRouteWritesOnlyWordsOfItsLists(t *testing.T) {
	p := testPage(t, appOrigin)
	var buf syncBuf
	p.Trace = &buf
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	base := srv.URL + "/event?t=" + p.token
	send(t, "POST", base+"&a=click&n=no_effect", appOrigin, "")
	send(t, "POST", base+"&a=click&n=hunter2", appOrigin, "")
	send(t, "POST", base+"&a=rm%20-rf&n=ok", appOrigin, "")
	send(t, "POST", srv.URL+"/event?a=click&n=ok", appOrigin, "")
	if got := buf.String(); got != "webscope: event page click no_effect\n" {
		t.Errorf("only the listed words with the token: %q", got)
	}
}

// syncBuf is a strings.Builder that two goroutines may write and read.
type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
