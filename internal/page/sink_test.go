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

// TestAnAnswerKeptByTheHelperIsGivenByPageResult: the helper posts the whole answer of a snippet to the listener, which needs the token and the origin of
// the app, and page(result) gives it back (in parts when it is long) until a new one replaces it.
func TestAnAnswerKeptByTheHelperIsGivenByPageResult(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	if out, err := askPage(t, p, "result", nil); err != nil || !strings.Contains(out, "no answer is kept") {
		t.Errorf("before any answer: %q, %v", out, err)
	}
	if res := send(t, "POST", srv.URL+"/result", appOrigin, "no token"); res.StatusCode < 400 {
		t.Errorf("a post with no token must be refused, got %d", res.StatusCode)
	}
	if res := send(t, "POST", srv.URL+"/result?t="+p.token, "http://evil.example", "other origin"); res.StatusCode < 400 {
		t.Errorf("a post from another origin must be refused, got %d", res.StatusCode)
	}
	long := strings.Repeat("línea de texto con ñandú\n", 600)
	if res := send(t, "POST", srv.URL+"/result?t="+p.token, appOrigin, long); res.StatusCode >= 300 {
		t.Fatalf("result %d", res.StatusCode)
	}
	first, err := askPage(t, p, "result", nil)
	if err != nil || len(first) > 8000+300 || !strings.HasPrefix(first, "línea de texto") || !strings.Contains(first, "offset") {
		t.Fatalf("a long answer comes in parts and says the offset of the next: %d bytes, %q, %v", len(first), first[max(0, len(first)-120):], err)
	}
	var got strings.Builder
	off := 0
	for i := 0; i < 20; i++ {
		part, err := askPage(t, p, "result", map[string]any{"offset": float64(off)})
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(`\n\(… (\d+) more bytes: result \{offset: (\d+)\}\)$`).FindStringSubmatch(part)
		if m == nil {
			got.WriteString(part)
			break
		}
		got.WriteString(strings.TrimSuffix(part, m[0]))
		off = atoi(m[2])
	}
	if got.String() != long {
		t.Errorf("the parts must add up to the answer: %d bytes of %d", got.Len(), len(long))
	}
	send(t, "POST", srv.URL+"/result?t="+p.token, appOrigin, "new one")
	if out, _ := askPage(t, p, "result", nil); out != "new one" {
		t.Errorf("a new answer replaces the old one: %q", out)
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// TestAnAnswerTooLongForJavascriptToolIsKeptAndShortened runs in a real Chrome: javascript_tool cuts what it returns at 1000 characters and blocks text with
// chained query values, so the helper keeps the whole answer in the listener and returns a short one.
func TestAnAnswerTooLongForJavascriptToolIsKeptAndShortened(t *testing.T) {
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
	p.SinkChars = 900
	load, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	steps := []string{load}
	for _, a := range []struct {
		action string
		args   map[string]any
	}{
		{"read", map[string]any{"selector": "#short"}},
		{"read", map[string]any{"selector": "#long", "max": float64(4000)}},
		{"read", map[string]any{"selector": "#qs"}},
	} {
		s, err := snippet(a.action, a.args)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, s)
	}
	raw, _ := json.Marshal(steps)
	mu.Lock()
	html_ = strings.Replace(strings.Replace(fixturePage, "{{STEPS}}", string(raw), 1), "<p id=long></p>", "<p id=long>"+strings.Repeat("palabra ", 400)+"</p>", 1)
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
	if out[1] != "texto corto" {
		t.Errorf("a short answer with no query values is returned as it is: %q", out[1])
	}
	if len([]rune(out[2])) > 1000 || !strings.Contains(out[2], "page(result)") || !strings.Contains(out[2], "more chars") {
		t.Errorf("a long answer is cut under 1000 characters and says where the rest is: %d chars: %q", len([]rune(out[2])), out[2][max(0, len(out[2])-200):])
	}
	if strings.Contains(out[3], "a=1") || !strings.Contains(out[3], "/x?a&b&c") || !strings.Contains(out[3], "query values hidden") {
		t.Errorf("chained query values are hidden in what javascript_tool sees: %q", out[3])
	}
	full, err := askPage(t, p, "result", nil)
	if err != nil || !strings.Contains(full, "/x?a=1&b=2&c=3") {
		t.Errorf("page(result) has the answer as it is, with the values: %q, %v", full, err)
	}
}
