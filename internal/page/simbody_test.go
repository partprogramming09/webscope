package page

import (
	"strings"
	"testing"
)

// TestSimulateAnswersWithTheBodyItIsGiven runs in a real Chrome: a simulated fetch and a simulated XMLHttpRequest get the body that simulate was given
// (an object or a text), and without one they get the short default message.
func TestSimulateAnswersWithTheBodyItIsGiven(t *testing.T) {
	app, mu, html_ := fixtureApp(t)
	p := testPage(t, app.URL)
	load, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	steps := []string{load}
	for _, a := range []map[string]any{
		{"match": "/api/zz", "status": 422, "body": map[string]any{"errors": map[string]any{"name": []any{"required"}}}},
		{"match": "/api/xx", "status": 409, "body": "plain text body"},
		{"match": "/api/yy", "status": 500},
		{"match": "/api/big", "status": 500, "body": strings.Repeat("x", 2001)},
	} {
		s, err := snippet("simulate", a)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, s)
	}
	steps = append(steps,
		`await fetch('/api/zz').then(async r=>r.status+' '+await r.text())`,
		`await new Promise(r=>{const x=new XMLHttpRequest();x.open('GET','/api/xx');x.onloadend=()=>r(x.status+' '+x.responseText);x.send()})`,
		`await fetch('/api/yy').then(async r=>r.status+' '+await r.text())`,
	)
	out := runSnippets(t, p, app, mu, html_, steps)
	if !strings.Contains(out[1], "with the body given") {
		t.Errorf("the answer says a body was set: %q", out[1])
	}
	if out[5] != `422 {"errors":{"name":["required"]}}` {
		t.Errorf("a simulated fetch answers with the object given: %q", out[5])
	}
	if out[6] != "409 plain text body" {
		t.Errorf("a simulated XMLHttpRequest answers with the text given: %q", out[6])
	}
	if out[7] != `500 {"message":"simulated by webscope"}` {
		t.Errorf("without a body the default message stays: %q", out[7])
	}
	if !strings.HasPrefix(out[4], "Failed: simulate body is 2001 characters") {
		t.Errorf("a body over 2000 characters is refused: %q", out[4])
	}
}
