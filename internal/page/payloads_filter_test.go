package page

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPayloadsFiltersByAddressAndBodyAndGivesOneByNumber: match and body keep only the requests that fit, every line has its number and the time it
// took, and index gives one request whole.
func TestPayloadsFiltersByAddressAndBodyAndGivesOneByNumber(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	for _, c := range []string{
		`{"m":"GET","u":"/api/tariffs?page=2","s":200,"res":"{\"rows\":3}","d":812}`,
		`{"m":"POST","u":"/api/orders","s":422,"req":"{\"name\":\"ZZ_a\"}","res":"{\"error\":\"bad\"}","d":40}`,
		`{"m":"GET","u":"/api/cities","s":200,"res":"x","d":5}`,
	} {
		send(t, "POST", srv.URL+"/capture?t="+p.token, appOrigin, c)
	}
	all, _ := askPage(t, p, "payloads", nil)
	if !strings.Contains(all, "#1 GET /api/tariffs?page=2 200 812 ms") || !strings.Contains(all, "#2 POST /api/orders 422 40 ms") {
		t.Errorf("each line has its number and its time: %q", all)
	}
	byURL, _ := askPage(t, p, "payloads", map[string]any{"match": `^get .*/tariffs`})
	if !strings.Contains(byURL, "1 match") || !strings.Contains(byURL, "#1 ") || strings.Contains(byURL, "orders") || strings.Contains(byURL, "cities") {
		t.Errorf("match keeps the requests whose method and address fit: %q", byURL)
	}
	byBody, _ := askPage(t, p, "payloads", map[string]any{"body": "zz_a"})
	if !strings.Contains(byBody, "1 match") || !strings.Contains(byBody, "#2 POST /api/orders") || strings.Contains(byBody, "tariffs") {
		t.Errorf("body keeps the requests whose sent or got text fits: %q", byBody)
	}
	one, _ := askPage(t, p, "payloads", map[string]any{"index": float64(2)})
	if !strings.HasPrefix(one, "#2 POST /api/orders 422 40 ms") || !strings.Contains(one, `sent: {"name":"ZZ_a"}`) || !strings.Contains(one, `got: {"error":"bad"}`) {
		t.Errorf("index gives that request with its bodies: %q", one)
	}
	if out, _ := askPage(t, p, "payloads", map[string]any{"index": float64(9)}); !strings.HasPrefix(out, "Failed: no request #9") {
		t.Errorf("a number past the last is refused: %q", out)
	}
	if out, _ := askPage(t, p, "payloads", map[string]any{"match": "("}); !strings.HasPrefix(out, "Failed: match is not a regular expression") {
		t.Errorf("a bad regex is refused: %q", out)
	}
}

// captured is a page with n captures sent through its listener, the k-th to /api/rK.
func captured(t *testing.T, n int) *Tool {
	t.Helper()
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	t.Cleanup(srv.Close)
	for k := 1; k <= n; k++ {
		send(t, "POST", srv.URL+"/capture?t="+p.token, appOrigin, fmt.Sprintf(`{"m":"GET","u":"/api/r%d","s":200}`, k))
	}
	return p
}

func TestPayloadsFirstLineIsTheMark(t *testing.T) {
	out, _ := askPage(t, captured(t, 3), "payloads", nil)
	if !strings.HasPrefix(out, "mark 3\n") {
		t.Fatalf("the answer starts with the number of the last capture:\n%s", out)
	}
}

func TestPayloadsAfterShowsOnlyNewer(t *testing.T) {
	out, _ := askPage(t, captured(t, 3), "payloads", map[string]any{"after": float64(2)})
	if !strings.Contains(out, "#3 GET /api/r3") || strings.Contains(out, "/api/r2") || strings.Contains(out, "/api/r1") {
		t.Fatalf("after 2 shows only #3:\n%s", out)
	}
}

func TestPayloadsAfterAddsLogLinesSinceTheMark(t *testing.T) {
	p := captured(t, 2)
	p.LogPath = filepath.Join(t.TempDir(), "app.log")
	os.WriteFile(p.LogPath, []byte("ERROR old\n"), 0o644)
	if out, _ := askPage(t, p, "payloads", nil); !strings.HasPrefix(out, "mark 2\n") {
		t.Fatalf("got %q", out)
	}
	f, _ := os.OpenFile(p.LogPath, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("ERROR boom\n")
	f.Close()
	out, _ := askPage(t, p, "payloads", map[string]any{"after": float64(2)})
	if !strings.Contains(out, "ERROR boom") || strings.Contains(out, "ERROR old") {
		t.Fatalf("after 2 shows the log lines written since mark 2, and only those:\n%s", out)
	}
}

func TestPayloadsAfterBeyondLast(t *testing.T) {
	out, _ := askPage(t, captured(t, 3), "payloads", map[string]any{"after": float64(99)})
	if !strings.Contains(out, "no requests after #99 (last is #3)") || strings.Contains(out, "Failed") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestPayloadsAfterCountsDroppedCaptures(t *testing.T) {
	p := captured(t, 55)
	all, _ := askPage(t, p, "payloads", map[string]any{"limit": float64(maxCaptures)})
	if !strings.HasPrefix(all, "mark 55\n") || !strings.Contains(all, "#6 GET /api/r6 ") || strings.Contains(all, "#1 ") {
		t.Fatalf("the oldest kept capture is #6:\n%s", all)
	}
	if one, _ := askPage(t, p, "payloads", map[string]any{"index": float64(6)}); !strings.HasPrefix(one, "#6 GET /api/r6 ") {
		t.Fatalf("index 6 is the sixth that came: %q", one)
	}
	if one, _ := askPage(t, p, "payloads", map[string]any{"index": float64(3)}); !strings.HasPrefix(one, "Failed: no request #3") {
		t.Fatalf("a dropped capture is gone: %q", one)
	}
	if out, _ := askPage(t, p, "payloads", map[string]any{"after": float64(53)}); !strings.Contains(out, "#54 ") || !strings.Contains(out, "#55 ") || strings.Contains(out, "#53 GET") {
		t.Fatalf("after counts by the global number:\n%s", out)
	}
}

func TestPayloadsAfterUnissuedMark(t *testing.T) {
	p := captured(t, 3)
	p.LogPath = filepath.Join(t.TempDir(), "app.log")
	os.WriteFile(p.LogPath, []byte("ERROR old\n"), 0o644)
	out, _ := askPage(t, p, "payloads", map[string]any{"after": float64(1)})
	if !strings.Contains(out, "#2 GET /api/r2") || !strings.Contains(out, "no log mark 1: log lines not read") || strings.Contains(out, "ERROR old") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestPayloadsAfterMustBeAWholeNumber(t *testing.T) {
	for _, v := range []any{float64(-1), 1.5, "x"} {
		if out, _ := askPage(t, captured(t, 1), "payloads", map[string]any{"after": v}); !strings.HasPrefix(out, "Failed: after must be a whole number") {
			t.Fatalf("after %v: %q", v, out)
		}
	}
}

// logged gives p a log file holding body and answers its path.
func logged(t *testing.T, p *Tool, body string) string {
	t.Helper()
	p.LogPath = filepath.Join(t.TempDir(), "app.log")
	os.WriteFile(p.LogPath, []byte(body), 0o644)
	return p.LogPath
}

func appendLog(t *testing.T, path, s string) {
	t.Helper()
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(s)
	f.Close()
}

func TestPayloadsAfterSaysWhichWereDropped(t *testing.T) {
	out, _ := askPage(t, captured(t, 55), "payloads", map[string]any{"after": float64(3), "limit": float64(maxCaptures)})
	if !strings.Contains(out, "requests #4 to #5 were dropped") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestPayloadsAfterCountsProblemsAfterTheMarkOnly(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	send(t, "POST", srv.URL+"/capture?t="+p.token, appOrigin, `{"m":"GET","u":"/api/a","s":500}`)
	send(t, "POST", srv.URL+"/capture?t="+p.token, appOrigin, `{"m":"GET","u":"/api/b","s":200}`)
	out, _ := askPage(t, p, "payloads", map[string]any{"after": float64(1)})
	if strings.Contains(out, "Problems") || !strings.Contains(out, "1 after #1") {
		t.Fatalf("a failure before the mark is not this click's:\n%s", out)
	}
}

func TestPayloadsAfterKeepsTheFirstPlaceOfAMark(t *testing.T) {
	p := captured(t, 2)
	path := logged(t, p, "")
	askPage(t, p, "payloads", nil)
	appendLog(t, path, "ERROR boom\n")
	askPage(t, p, "payloads", nil) // the same mark again, after the click
	if out, _ := askPage(t, p, "payloads", map[string]any{"after": float64(2)}); !strings.Contains(out, "ERROR boom") {
		t.Fatalf("mark 2 keeps where the log was the first time:\n%s", out)
	}
}

func TestPayloadsAfterWithAShorterLogDoesNotClaimLines(t *testing.T) {
	p := captured(t, 2)
	path := logged(t, p, "ERROR old one\nERROR old two\n")
	askPage(t, p, "payloads", nil)
	os.WriteFile(path, []byte("ERROR x\n"), 0o644)
	out, _ := askPage(t, p, "payloads", map[string]any{"after": float64(2)})
	if strings.Contains(out, "ERROR") || !strings.Contains(out, "the log is shorter than at mark 2") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestPayloadsAfterKeepsTheLogWhenTheListIsCut(t *testing.T) {
	p := testPage(t, appOrigin)
	srv := httptest.NewServer(p.handler())
	defer srv.Close()
	path := logged(t, p, "")
	askPage(t, p, "payloads", nil)
	big := strings.Repeat("x", 3000)
	for range 20 {
		send(t, "POST", srv.URL+"/capture?t="+p.token, appOrigin, fmt.Sprintf(`{"m":"POST","u":"/api/a","s":200,"req":"%s","res":"%s"}`, big, big))
	}
	appendLog(t, path, "ERROR boom\n")
	out, _ := askPage(t, p, "payloads", map[string]any{"after": float64(0), "limit": float64(20)})
	if !strings.Contains(out, "more requests not shown") || !strings.Contains(out, "ERROR boom") {
		t.Fatalf("the log lines stay when the list is cut:\n%s", out[max(0, len(out)-400):])
	}
}

func TestPayloadsAfterNullIsAbsent(t *testing.T) {
	if out, _ := askPage(t, captured(t, 1), "payloads", map[string]any{"after": nil}); strings.HasPrefix(out, "Failed") {
		t.Fatalf("got %q", out)
	}
}
