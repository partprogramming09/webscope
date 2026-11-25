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

func TestAPortWildcardIsAcceptedOnlyForLocalhostAndLoopbackAsTheHumanWroteIt(t *testing.T) {
	accepted := []struct{ appURL, origin string }{
		{"http://localhost:*", "http://localhost:5180"},
		{"http://localhost:*", "http://localhost:5173"},
		{"http://127.0.0.1:*", "http://127.0.0.1:5180"},
		{"http://localhost:*", "http://localhost"},
		{"http://other.example, http://localhost:*", "http://localhost:5180"},
		{"HTTP://LocalHost:*", "http://localhost:5180"},
	}
	for _, c := range accepted {
		if !testPage(t, c.appURL).allows(c.origin) {
			t.Errorf("%q must accept %q", c.appURL, c.origin)
		}
	}
	rejected := []struct{ appURL, origin string }{
		{"http://localhost:*", "http://evil.localhost.example:5180"},
		{"http://localhost:*", "http://localhost.evil.com"},
		{"http://localhost:*", "http://localhost.evil.com:5180"},
		{"http://localhost:*", "http://evil.com"},
		{"http://localhost:*", "http://127.0.0.1:5180"},
		{"http://localhost:*", "https://localhost:5180"},
		{"http://localhost:*", "http://localhost:5180x"},
		{"http://localhost:*", "http://localhost:"},
		{"http://localhost:*", "http://localhost:51800000"},
		{"http://localhost:*", "http://user@localhost:5180"},
		{"http://localhost:5173", "http://localhost:5180"},
		{"http://other.example:*", "http://other.example:5180"},
		{"https://localhost:*", "https://localhost:5180"},
		{"http://*:5180", "http://localhost:5180"},
		{"http://*", "http://evil.com"},
		{"*", "http://localhost:5180"},
		{"http://localhost.evil.com:*", "http://localhost.evil.com:5180"},
		{"http://evil.localhost.example:*", "http://evil.localhost.example:5180"},
	}
	for _, c := range rejected {
		if testPage(t, c.appURL).allows(c.origin) {
			t.Errorf("%q must refuse %q", c.appURL, c.origin)
		}
	}
}

func TestTheListenerAnswersAWildcardOriginAndRefusesOthers(t *testing.T) {
	p := testPage(t, "http://localhost:*")
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	url := srv.URL + "/sim?t=" + p.token + "&status=503"
	if res := send(t, "PUT", url, "http://localhost:5180", ""); res.StatusCode != 503 || res.Header.Get("Access-Control-Allow-Origin") != "http://localhost:5180" {
		t.Errorf("a localhost port: %d, ACAO %q", res.StatusCode, res.Header.Get("Access-Control-Allow-Origin"))
	}
	for _, o := range []string{"http://evil.localhost.example:5180", "http://localhost.evil.com", "http://evil.example"} {
		if res := send(t, "PUT", url, o, ""); res.StatusCode != http.StatusForbidden {
			t.Errorf("%q: %d, want 403", o, res.StatusCode)
		}
	}
}

// The helper in the page compares the tab's origin with the list too: a wildcard must not raise the warning for a localhost tab.
func TestTheHelperDoesNotWarnForATabMatchedByAWildcard(t *testing.T) {
	var mu sync.Mutex
	var html_ string
	app := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		io.WriteString(w, html_)
	}))
	app.Start()
	defer app.Close()
	run := func(appURL string) string {
		p := testPage(t, appURL)
		load, err := askPage(t, p, "load", nil)
		if err != nil {
			t.Fatal(err)
		}
		state, _ := snippet("state", nil)
		raw, _ := json.Marshal([]string{load, state})
		mu.Lock()
		html_ = strings.Replace(auditPage, "{{STEPS}}", string(raw), 1)
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
			t.Fatalf("the page did not finish its steps")
		}
		var out []string
		if err := json.Unmarshal([]byte(html.UnescapeString(string(m[1]))), &out); err != nil || len(out) != 2 {
			t.Fatalf("results %q: %v", m[1], err)
		}
		return out[1]
	}
	if out := run("http://127.0.0.1:*"); strings.Contains(out, "Warning") {
		t.Errorf("a 127.0.0.1 tab under http://127.0.0.1:* must not be warned:\n%s", out)
	}
	if out := run("http://localhost:*"); !strings.Contains(out, "Warning: This tab is "+app.URL) {
		t.Errorf("a 127.0.0.1 tab is not under http://localhost:*, it must be warned:\n%s", out)
	}
}
