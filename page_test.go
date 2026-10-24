package main

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

const appOrigin = "http://localhost:5173"

// testPage is a page tool whose listener (if any) stops with the test; life is set before any use, as main does.
func testPage(t *testing.T, appURL string) *pageTool {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p := newPage(appURL)
	p.life = ctx
	return p
}

func askPage(t *testing.T, p *pageTool, action string, args map[string]any) (string, error) {
	t.Helper()
	return p.answer(pageArgs{Action: action, Args: args})
}

func send(t *testing.T, method, url, origin, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if method == http.MethodOptions {
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "content-type")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func TestSnippetCarriesArgsOnlyAsJSON(t *testing.T) {
	nasty := map[string]any{"text": "a\"b\nc</script>ñandú 😀  ", "code": "007", "n": 7}
	want, _ := json.Marshal(nasty)
	for _, action := range []string{"state", "read", "fill", "click", "press", "simulate"} {
		got, err := snippet(action, nasty)
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		if !strings.Contains(got, string(want)) {
			t.Errorf("%s: the arguments are not in the snippet as JSON: %q", action, got)
		}
		if strings.Contains(got, "</script>") || strings.ContainsAny(got, "\n\r  ") {
			t.Errorf("%s: raw </script> or line break in the snippet: %q", action, got)
		}
		if !strings.Contains(got, `"007"`) || !strings.Contains(got, "window.__ws."+action+"(") {
			t.Errorf("%s: 007 lost its quotes or the call is not there: %q", action, got)
		}
	}
	if got, _ := snippet("state", nil); !strings.Contains(got, "({})") {
		t.Errorf("no arguments must be an empty object: %q", got)
	}
}

func TestSnippetRefusesAnythingButTheActionList(t *testing.T) {
	for _, action := range []string{"", "state);alert(1);//", "payloads", "help", "load", "eval"} {
		if got, err := snippet(action, nil); err == nil {
			t.Errorf("snippet(%q) = %q, want an error", action, got)
		}
	}
}

func TestHelpIsTextAndNamesEveryAction(t *testing.T) {
	out, err := askPage(t, testPage(t, appOrigin), "help", nil)
	if err != nil || strings.Contains(out, "window.__ws") {
		t.Fatalf("%q, %v", out, err)
	}
	for _, a := range []string{"load", "state", "read", "fill", "click", "press", "payloads", "simulate"} {
		if !strings.Contains(out, a) {
			t.Errorf("help does not mention %s", a)
		}
	}
}

func TestLoadNeedsTheAppURLAndAnUnknownActionNamesTheList(t *testing.T) {
	_, err := askPage(t, testPage(t, ""), "load", nil)
	if err == nil || !strings.Contains(err.Error(), "WEBSCOPE_APP_URL") {
		t.Errorf("load without app url: %v", err)
	}
	_, err = askPage(t, testPage(t, appOrigin), "frobnicate", nil)
	if err == nil || !strings.Contains(err.Error(), "fill") {
		t.Errorf("unknown action: %v", err)
	}
}

func TestLoadIsOneShortLineWithPortAndToken(t *testing.T) {
	p := testPage(t, appOrigin)
	out, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\n") || len(out) > 400 || !strings.Contains(out, "http://127.0.0.1:") || !strings.Contains(out, "/page.js?t="+p.token) {
		t.Fatalf("bootstrap = %q", out)
	}
}

func TestEveryPageHasItsOwnToken(t *testing.T) {
	a, b := newPage(appOrigin), newPage(appOrigin)
	if len(a.token) < 32 || a.token == b.token {
		t.Fatalf("tokens %q and %q", a.token, b.token)
	}
}

func TestTheListenerStopsWhenTheContextEnds(t *testing.T) {
	p := newPage(appOrigin)
	ctx, cancel := context.WithCancel(context.Background())
	p.life = ctx
	out, err := p.answer(pageArgs{Action: "load"})
	if err != nil {
		t.Fatal(err)
	}
	from, to := strings.Index(out, "http://127.0.0.1:"), strings.Index(out, "/page.js")
	if from < 0 || to < from {
		t.Fatalf("no listener address in %q", out)
	}
	url := out[from:to] + "/page.js?t=" + p.token
	if res, err := http.Get(url); err != nil || res.StatusCode != 200 {
		t.Fatalf("not serving before the cancel: %v", err)
	}
	cancel()
	for i := 0; i < 40; i++ {
		if _, err := http.Get(url); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("still serving after the context ended")
}

func TestTheServerNeedsTheTokenForEveryRoute(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	for _, path := range []string{"/page.js", "/page.js?t=wrong", "/capture", "/capture?t=wrong", "/sim?status=500", "/sim?status=500&t=wrong"} {
		for _, method := range []string{"GET", "POST", "OPTIONS"} {
			if res := send(t, method, srv.URL+path, appOrigin, "{}"); res.StatusCode != http.StatusForbidden {
				t.Errorf("%s %s = %d, want 403", method, path, res.StatusCode)
			}
		}
	}
	res := send(t, "GET", srv.URL+"/page.js?t="+p.token, "", "")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "javascript") || len(body) == 0 {
		t.Errorf("with the token: %d %q, %d bytes", res.StatusCode, res.Header.Get("Content-Type"), len(body))
	}
}

func TestTheServerAnswersOnlyTheAppOrigin(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	url := srv.URL + "/capture?t=" + p.token
	for _, method := range []string{"POST", "OPTIONS"} {
		res := send(t, method, url, "http://evil.example", `{"m":"GET","u":"/x","s":200}`)
		if res.StatusCode != http.StatusForbidden || res.Header.Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("%s from another origin: %d, ACAO %q", method, res.StatusCode, res.Header.Get("Access-Control-Allow-Origin"))
		}
		res = send(t, method, url, appOrigin, `{"m":"GET","u":"/x","s":200}`)
		if res.StatusCode >= 300 || res.Header.Get("Access-Control-Allow-Origin") != appOrigin {
			t.Errorf("%s from the app: %d, ACAO %q", method, res.StatusCode, res.Header.Get("Access-Control-Allow-Origin"))
		}
	}
	if res := send(t, "OPTIONS", url, appOrigin, ""); !strings.Contains(res.Header.Get("Access-Control-Allow-Methods"), "POST") || !strings.Contains(strings.ToLower(res.Header.Get("Access-Control-Allow-Headers")), "content-type") {
		t.Errorf("preflight does not allow POST with content-type: %v", res.Header)
	}
	if out, _ := askPage(t, p, "payloads", nil); strings.Contains(out, "evil") || strings.Count(out, "/x") != 1 {
		t.Errorf("only the request from the app is kept, got %q", out)
	}
}

func TestTheServerCutsWhatIsTooBig(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	big := `{"m":"POST","u":"/x","s":200,"req":"` + strings.Repeat("a", 300_000) + `"}`
	if res := send(t, "POST", srv.URL+"/capture?t="+p.token, appOrigin, big); res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status %d, want 413", res.StatusCode)
	}
	if out, _ := askPage(t, p, "payloads", nil); strings.Contains(out, "/x") {
		t.Errorf("a cut request was stored: %q", out)
	}
}

func TestPayloadsShowWhatWasCapturedCappedAndIntact(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	if out, _ := askPage(t, p, "payloads", nil); !strings.Contains(out, "0 requests") {
		t.Errorf("empty = %q", out)
	}
	post := func(body string) {
		if res := send(t, "POST", srv.URL+"/capture?t="+p.token, appOrigin, body); res.StatusCode >= 300 {
			t.Fatalf("capture %d", res.StatusCode)
		}
	}
	post(`{"m":"POST","u":"/api/orders","s":422,"req":"{\"code\":\"007\",\"n\":\"ñandú 😀\"}","res":"{\"message\":\"bad\\tvalue\"}"}`)
	out, _ := askPage(t, p, "payloads", nil)
	for _, want := range []string{"POST /api/orders 422", `"code":"007"`, "ñandú 😀", "bad"} {
		if !strings.Contains(out, want) {
			t.Errorf("payloads lacks %q: %q", want, out)
		}
	}
	for i := 0; i < 70; i++ {
		post(`{"m":"GET","u":"/n/` + strings.Repeat("0", i%3) + string(rune('A'+i%26)) + `","s":200,"res":"` + strings.Repeat("x", 5000) + `"}`)
	}
	out, _ = askPage(t, p, "payloads", nil)
	if strings.Count(out, " 200\n") > 50 || len(out) > 20_000 || strings.Contains(out, "/api/orders") {
		t.Errorf("not capped: %d bytes, %d entries, oldest kept: %v", len(out), strings.Count(out, " 200\n"), strings.Contains(out, "/api/orders"))
	}
}

func TestSimEndpointAnswersAnErrorStatusToTheAppOnly(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	base := srv.URL + "/sim?t=" + p.token + "&status="
	res := send(t, "PUT", base+"503", appOrigin, "")
	if res.StatusCode != 503 || res.Header.Get("Access-Control-Allow-Origin") != appOrigin {
		t.Errorf("503: %d, ACAO %q", res.StatusCode, res.Header.Get("Access-Control-Allow-Origin"))
	}
	for _, status := range []string{"200", "302", "99", "600", "abc", ""} {
		if res := send(t, "GET", base+status, appOrigin, ""); res.StatusCode != http.StatusBadRequest {
			t.Errorf("status=%q gave %d, want 400", status, res.StatusCode)
		}
	}
	if res := send(t, "GET", base+"500", "http://evil.example", ""); res.StatusCode != http.StatusForbidden {
		t.Errorf("another origin: %d", res.StatusCode)
	}
}

// chromePath finds a Chrome or Chromium; the JavaScript can only be tested in one, so a missing browser fails the test (it is not skipped).
func chromePath(t *testing.T) string {
	t.Helper()
	cands := []string{os.Getenv("WEBSCOPE_CHROME"), "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "google-chrome", "google-chrome-stable", "chromium"}
	for _, c := range cands {
		if c == "" {
			continue
		}
		if path, err := exec.LookPath(c); err == nil {
			return path
		}
	}
	t.Fatal("no Chrome found: install one or set WEBSCOPE_CHROME")
	return ""
}

const fixturePage = `<!doctype html><meta charset=utf-8><title>Pedido</title><h1>Nuevo pedido</h1>
<form id=f><label for=n>Nombre</label><input id=n name=n>
<label for=c>Código</label><input id=c name=c>
<label><input type=checkbox id=a> Acepto</label>
<label for=m>Estado</label><input id=m value=ENPROCESO>
<label for=k>Tipo</label><select id=k><option>uno<option>dos</select>
<button type=button id=go>Guardar</button><button type=button id=up>Actualizar</button><button type=button id=big>Grande</button><button type=button>Cancelar</button></form>
<p role=alert id=al></p><p id=mirror></p><p id=picked></p><pre id=out></pre>
<script type=application/json id=steps>{{STEPS}}</script>
<script>
go.onclick=()=>fetch('/api/orders',{method:'POST',body:JSON.stringify({n:n.value,c:c.value,a:a.checked,k:k.value})}).then(r=>al.textContent='status '+r.status)
big.onclick=()=>fetch('/api/big').then(r=>r.text()).then(t=>al.textContent='big '+t.length)
up.onclick=()=>{const x=new XMLHttpRequest();x.open('PUT','/api/orders/1');x.onloadend=()=>al.textContent='xhr '+x.status;x.send('{}')}
n.oninput=()=>mirror.textContent=n.value // what a v-model would copy: it only hears the input and change events
k.onchange=()=>picked.textContent=k.value
m.onblur=()=>{m.value='ENPROCESO'} // a v-select keeps its model and puts it back when the field loses focus
n.onkeydown=e=>{if(e.key==='Escape')al.textContent='esc'}
const steps=JSON.parse(document.getElementById('steps').textContent),res=[]
const sleep=ms=>new Promise(r=>setTimeout(r,ms))
;(async()=>{for(const s of steps){try{res.push(String((0,eval)(s)))}catch(e){res.push('THROW '+e)}await sleep(300)}out.textContent=JSON.stringify(res)})()
</script>`

// TestActionsRunInARealChrome loads a form from one origin, the webscope listener is another one (so CORS is real), runs every action the way
// javascript_tool would (an eval in the page) and reads the results back from the dumped DOM.
func TestActionsRunInARealChrome(t *testing.T) {
	var mu sync.Mutex
	var html_ string
	var posts, puts []string
	app := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		switch {
		case r.URL.Path == "/":
			io.WriteString(w, html_)
		case r.URL.Path == "/api/orders" && r.Method == "POST":
			posts = append(posts, string(b))
			w.WriteHeader(201)
			io.WriteString(w, `{"id":1}`)
		case r.URL.Path == "/api/big":
			io.WriteString(w, strings.Repeat("x", 100_000))
		case r.Method == "PUT":
			puts = append(puts, r.URL.Path)
			io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	app.Start()
	defer app.Close()
	p := testPage(t, app.URL)
	load, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	p2 := testPage(t, app.URL) // the MCP restarted: same app, new port and token
	load2, err := askPage(t, p2, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	nasty := "ñandú 😀 \"q\" </script>"
	steps := []string{load} // 0
	add := func(action string, args map[string]any) {
		s, err := snippet(action, args)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, s)
	}
	add("state", nil)                                                                                                                   // 1
	add("fill", map[string]any{"fields": map[string]any{"Nombre": nasty, "Código": "007", "Acepto": true, "Tipo": "dos", "Nada": "x"}}) // 2
	add("click", map[string]any{"text": "Guardar"})                                                                                     // 3
	add("state", nil)                                                                                                                   // 4
	add("simulate", map[string]any{"match": "/api/orders", "status": 503, "times": 1})                                                  // 5
	add("click", map[string]any{"text": "Guardar"})                                                                                     // 6
	add("read", map[string]any{"selector": "#al"})                                                                                      // 7
	add("simulate", map[string]any{"match": "/api/orders/1", "status": 500, "times": 1})                                                // 8
	add("click", map[string]any{"selector": "#up"})                                                                                     // 9
	add("read", map[string]any{"selector": "#al"})                                                                                      // 10
	add("press", map[string]any{"key": "Escape", "selector": "#n"})                                                                     // 11
	add("read", map[string]any{"selector": "#al"})                                                                                      // 12
	add("click", map[string]any{"text": "No existe"})                                                                                   // 13
	add("read", map[string]any{"selector": "#nada"})                                                                                    // 14
	add("read", map[string]any{"selector": "#mirror"})                                                                                  // 15
	add("read", map[string]any{"selector": "#picked"})                                                                                  // 16
	add("click", map[string]any{"selector": "#big"})                                                                                    // 17
	add("read", map[string]any{"selector": "#al"})                                                                                      // 18
	steps = append(steps, load2)                                                                                                        // 19
	add("click", map[string]any{"text": "Guardar"})                                                                                     // 20
	add("fill", map[string]any{"fields": map[string]any{"Estado": "Rechazada"}})                                                        // 21
	raw, _ := json.Marshal(steps)
	mu.Lock()
	html_ = strings.Replace(fixturePage, "{{STEPS}}", string(raw), 1)
	mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	flags := []string{"--headless=new", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--user-data-dir=" + t.TempDir(), "--virtual-time-budget=20000", "--dump-dom"}
	if os.Getenv("CI") != "" {
		flags = append(flags, "--no-sandbox") // hosted Linux runners forbid the sandbox's user namespaces; the page is this test's own fixture
	}
	cmd := exec.CommandContext(ctx, chromePath(t), append(flags, app.URL+"/")...)
	dom, err := cmd.Output()
	if err != nil {
		t.Fatalf("chrome: %v", err)
	}
	m := regexp.MustCompile(`(?s)<pre id="out">(.*?)</pre>`).FindSubmatch(dom)
	if m == nil {
		t.Fatalf("the page did not finish its steps; DOM tail: %.600s", dom[max(0, len(dom)-600):])
	}
	var out []string
	if err := json.Unmarshal([]byte(html.UnescapeString(string(m[1]))), &out); err != nil || len(out) != len(steps) {
		t.Fatalf("results %q: %v", m[1], err)
	}
	has := func(i int, wants ...string) {
		t.Helper()
		for _, w := range wants {
			if !strings.Contains(out[i], w) {
				t.Errorf("step %d lacks %q: %q", i, w, out[i])
			}
		}
	}
	for i, o := range out {
		if strings.HasPrefix(o, "THROW") {
			t.Errorf("step %d threw: %s", i, o)
		}
	}
	has(1, "Nuevo pedido", "Nombre", "Acepto", "Tipo", "Guardar", "Cancelar")
	has(2, "Nada: not found", "Nombre: ok", "Código: ok", "Acepto: ok", "Tipo: ok")
	has(4, "ñandú 😀 \"q\" </script>", `"007"`, "status 201")
	has(7, "status 503")
	has(10, "xhr 500")
	has(12, "esc")
	has(13, "Failed", "No existe")
	has(14, "Failed", "#nada")
	if out[15] != nasty || out[16] != "dos" {
		t.Errorf("the page did not hear the input and change events: mirror %q, picked %q", out[15], out[16])
	}
	mu.Lock()
	defer mu.Unlock()
	has(0, "loading")
	has(18, "big 100000")
	has(19, "loading")
	has(21, "Estado: ", "ENPROCESO", "Rechazada")
	if strings.Contains(out[21], "Estado: ok") {
		t.Errorf("fill said ok but the page put the old value back: %q", out[21])
	}
	if len(posts) != 2 || len(puts) != 0 {
		t.Fatalf("the app saw %d posts and %d puts; the simulated ones must not reach it", len(posts), len(puts))
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(posts[0]), &sent); err != nil || sent["n"] != nasty || sent["c"] != "007" || sent["a"] != true || sent["k"] != "dos" {
		t.Errorf("what the app received: %s (%v)", posts[0], err)
	}
	seen, _ := askPage(t, p, "payloads", map[string]any{"limit": 50})
	for _, want := range []string{"POST /api/orders 201", "POST /api/orders 503", "PUT /api/orders/1 500", `"c":"007"`, "GET /api/big 200"} {
		if !strings.Contains(seen, want) {
			t.Errorf("payloads lacks %q: %s", want, seen)
		}
	}
	if strings.Count(seen, "POST /api/orders") != 2 {
		t.Errorf("the first server must hold only the two requests made before the second load: %s", seen)
	}
	if after, _ := askPage(t, p2, "payloads", nil); !strings.Contains(after, "POST /api/orders 201") || strings.Count(after, "POST /api/orders") != 1 {
		t.Errorf("after a second load the captures go to the new server: %s", after)
	}
}
