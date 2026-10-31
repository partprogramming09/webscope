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
	list := steps(load)
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

func TestAFailedRequestBringsTheLastErrorLinesOfTheBackendLogNextToTheClick(t *testing.T) {
	logPath := t.TempDir() + "/php.log"
	os.WriteFile(logPath, []byte("[10:00:00] GET /api/ok 200\n[10:00:01] PHP Fatal error:  Uncaught Error: Call to undefined function foo() in /app/Export.php:12\n[10:00:02] GET /api/ok 200\n"), 0o600)
	t.Setenv("WEBSCOPE_LOG", logPath)
	app := func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/api/boom":
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
	if !strings.Contains(out[3], "Backend log") || !strings.Contains(out[3], "Fatal error") || strings.Contains(out[3], "GET /api/ok 200") {
		t.Errorf("a 500 must bring the error line of the log and only that: %q", out[3])
	}
	if seen, _ := askPage(t, p, "payloads", nil); !strings.Contains(seen, "Backend log") || !strings.Contains(seen, "Fatal error") {
		t.Errorf("payloads with a 500 must bring the log lines too: %s", seen)
	}
}

func TestTheLogRouteNeedsTheTokenAndAnswersNothingWithoutAConfiguredLog(t *testing.T) {
	t.Setenv("WEBSCOPE_LOG", "")
	p := testPage(t, appOrigin)
	if got := p.logLines(); got != "" {
		t.Errorf("no log configured must give nothing: %q", got)
	}
}
