package page

import (
	"strings"
	"testing"
)

// TestParityMapSaysHowManyNamesMatchedAndWhichDidNot: a map renames the names of a, and the answer says how many entries found their name on both sides, and
// names the ones that found it on neither or on only one.
func TestParityMapSaysHowManyNamesMatchedAndWhichDidNot(t *testing.T) {
	p := testPage(t, appOrigin)
	if _, err := askPage(t, p, "load", nil); err != nil {
		t.Fatal(err)
	}
	postCapture(t, p, Capture{M: "POST", U: "/a.php", S: 200, Req: `{"cliente":"x","valor":"1","sobra":"z"}`, Res: `{"ok":true}`})
	postCapture(t, p, Capture{M: "POST", U: "/api/b", S: 200, Req: `{"client":"x","value":"1"}`, Res: `{"ok":true}`})
	out, err := askPage(t, p, "parity", map[string]any{"req_a": "POST a.php", "req_b": "POST /api/b", "map": map[string]any{"cliente": "client", "valor": "valor_mal", "nada": "value"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"map: 1 of 3 names matched", "«valor» → «valor_mal»: not in b", "«nada»: not in a"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	out, err = askPage(t, p, "parity", map[string]any{"req_a": "POST a.php", "req_b": "POST /api/b", "map": map[string]any{"cliente": "client", "valor": "value"}})
	if err != nil || !strings.Contains(out, "map: 2 of 2 names matched") || strings.Contains(out, "not in") {
		t.Errorf("all matched must say so and name nothing: %q, %v", out, err)
	}
	out, _ = askPage(t, p, "parity", map[string]any{"req_a": "POST a.php", "req_b": "POST /api/b"})
	if strings.Contains(out, "map:") {
		t.Errorf("no map, no line about it: %q", out)
	}
}
