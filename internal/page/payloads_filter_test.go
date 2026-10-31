package page

import (
	"net/http/httptest"
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
