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
