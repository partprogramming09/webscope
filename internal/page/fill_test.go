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

// runSnippets loads the fixture page in a real Chrome, runs the snippets of the page tool in order (each after the helper's scripts finished loading) and
// returns what each gave. The first snippet is page(load).
func runSnippets(t *testing.T, p *Tool, app *httptest.Server, mu *sync.Mutex, html_ *string, steps []string) []string {
	t.Helper()
	raw, _ := json.Marshal(steps)
	mu.Lock()
	*html_ = strings.Replace(fixturePage, "{{STEPS}}", string(raw), 1)
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
	return out
}

// fixtureApp serves the page that the test sets in html_.
func fixtureApp(t *testing.T) (*httptest.Server, *sync.Mutex, *string) {
	t.Helper()
	var mu sync.Mutex
	html_ := new(string)
	app := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		io.WriteString(w, *html_)
	}))
	app.Start()
	t.Cleanup(app.Close)
	return app, &mu, html_
}

// TestFillPicksTheNthOfSeveralFieldsWithTheSameName runs in a real Chrome: «Nota» names a field of the page and one of a dialog (the dialog's comes
// first); «Nota#2» is the second, and a number past the last is not found.
func TestFillPicksTheNthOfSeveralFieldsWithTheSameName(t *testing.T) {
	app, mu, html_ := fixtureApp(t)
	p := testPage(t, app.URL)
	load, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	steps := []string{load}
	for _, a := range []map[string]any{
		{"fields": map[string]any{"Nota": "uno"}},
		{"fields": map[string]any{"Nota#2": "dos"}},
		{"fields": map[string]any{"Nota#3": "tres"}},
	} {
		s, err := snippet("fill", a)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, s)
		if len(steps) == 3 { // after the second fill: which field heard it
			r, _ := snippet("read", map[string]any{"selector": "#nn"})
			steps = append(steps, r)
		}
	}
	out := runSnippets(t, p, app, mu, html_, steps)
	if !strings.Contains(out[1], "Nota: ok") || !strings.Contains(out[1], "2 fields answer") || !strings.Contains(out[1], "#2") {
		t.Errorf("a name that fits two fields warns and says how to pick the second: %q", out[1])
	}
	if !strings.Contains(out[2], "Nota#2: ok") || strings.Contains(out[2], "warning") || out[3] != "outside" {
		t.Errorf("Nota#2 fills the field of the page (the one that tells «outside») with no warning: %q, then %q", out[2], out[3])
	}
	if !strings.Contains(out[4], "Nota#3: not found") {
		t.Errorf("a number past the last field is not found: %q", out[4])
	}
}
