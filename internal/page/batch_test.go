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

// TestBatchIsOneSnippetAndRefusesWhatNeedsTheServer: the steps travel as JSON in one snippet, and an action that the server answers by itself, or an
// unknown one, is refused before anything runs.
func TestBatchIsOneSnippetAndRefusesWhatNeedsTheServer(t *testing.T) {
	s, err := snippet("batch", map[string]any{"steps": []any{
		map[string]any{"action": "fill", "args": map[string]any{"Nombre": "</script>"}},
		map[string]any{"action": "state"},
	}})
	if err != nil || !strings.Contains(s, "window.__ws.batch(") || strings.Contains(s, "</script>") {
		t.Errorf("one snippet, with the arguments as JSON: %q, %v", s, err)
	}
	for name, args := range map[string]map[string]any{
		"no steps":      {},
		"empty steps":   {"steps": []any{}},
		"payloads":      {"steps": []any{map[string]any{"action": "payloads"}}},
		"load":          {"steps": []any{map[string]any{"action": "load"}}},
		"nested batch":  {"steps": []any{map[string]any{"action": "batch"}}},
		"unknown":       {"steps": []any{map[string]any{"action": "dance"}}},
		"not an object": {"steps": []any{"click"}},
	} {
		if _, err := snippet("batch", args); err == nil || !strings.Contains(err.Error(), "batch") {
			t.Errorf("%s must be refused naming batch: %v", name, err)
		}
	}
}

// TestBatchRunsTheStepsInARealChromeAndStopsAtTheFirstFailure runs in a real Chrome: the answers of the steps come back numbered in one text, a failed
// step stops the rest unless keepGoing, and a step that does not exist in the page fails by name.
func TestBatchRunsTheStepsInARealChromeAndStopsAtTheFirstFailure(t *testing.T) {
	var mu sync.Mutex
	var html_ string
	app := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		io.WriteString(w, html_)
	}))
	app.Start()
	defer app.Close()
	p := testPage(t, app.URL)
	load, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	steps := []string{load}
	for _, a := range []map[string]any{
		{"steps": []any{
			map[string]any{"action": "fill", "args": map[string]any{"Nombre": "Ana"}},
			map[string]any{"action": "read", "args": map[string]any{"selector": "#mirror"}},
		}},
		{"steps": []any{
			map[string]any{"action": "click", "args": map[string]any{"text": "No existe"}},
			map[string]any{"action": "read", "args": map[string]any{"selector": "#mirror"}},
		}},
		{"keepGoing": true, "steps": []any{
			map[string]any{"action": "click", "args": map[string]any{"text": "No existe"}},
			map[string]any{"action": "read", "args": map[string]any{"selector": "#mirror"}},
		}},
	} {
		s, err := snippet("batch", a)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, s)
	}
	raw, _ := json.Marshal(steps)
	mu.Lock()
	html_ = strings.Replace(fixturePage, "{{STEPS}}", string(raw), 1)
	mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	flags := []string{"--headless=new", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--user-data-dir=" + t.TempDir(), "--virtual-time-budget=90000", "--dump-dom"}
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
	if err := json.Unmarshal([]byte(html.UnescapeString(string(m[1]))), &out); err != nil || len(out) != len(steps) {
		t.Fatalf("results %q: %v", m[1], err)
	}
	if !strings.Contains(out[1], "1. fill: Nombre: ok") || !strings.Contains(out[1], "2. read: Ana") {
		t.Errorf("both steps run and come back numbered: %q", out[1])
	}
	if !strings.Contains(out[2], "1. click: Failed") || strings.Contains(out[2], "2. read") || !strings.Contains(out[2], "not run") {
		t.Errorf("a failed step stops the rest and says so: %q", out[2])
	}
	if !strings.Contains(out[3], "1. click: Failed") || !strings.Contains(out[3], "2. read: Ana") {
		t.Errorf("keepGoing runs the steps after a failure: %q", out[3])
	}
}
