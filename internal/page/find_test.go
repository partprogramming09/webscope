package page

import (
	"strings"
	"testing"
)

// TestFindListsOnlyWhatMatchesAndRefsPickElements runs in a real Chrome: find gives the matching fields and buttons with a ref and where they are, and
// fill and click take that ref when a name does not say which element.
func TestFindListsOnlyWhatMatchesAndRefsPickElements(t *testing.T) {
	app, mu, html_ := fixtureApp(t)
	p := testPage(t, app.URL)
	load, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	steps := []string{load}
	for _, s := range []struct {
		action string
		args   map[string]any
	}{
		{"find", map[string]any{"text": "nota"}},
		{"find", map[string]any{"regex": "^guard"}},
		{"find", map[string]any{"text": "zzz nothing"}},
		{"find", map[string]any{}},
		{"fill", map[string]any{"fields": map[string]any{"f13": "alfa"}}},
		{"find", map[string]any{"text": "alfa"}},
		{"click", map[string]any{"ref": "b36"}},
		{"click", map[string]any{"ref": "b999"}},
		{"fill", map[string]any{"fields": map[string]any{"Origen": "PAIS 250"}}},
		{"find", map[string]any{"text": "pais 250"}},
		{"fill", map[string]any{"fields": map[string]any{"css:[name=c]": "por css"}}},
		{"find", map[string]any{"text": "por css"}},
		{"fill", map[string]any{"fields": map[string]any{"css:[name=nada]": "x", "css:((": "y"}}},
		{"find", map[string]any{"text": "/^guard|^cancel/"}},
		{"find", map[string]any{"text": "Guardar|Cancelar"}},
		{"find", map[string]any{"text": "/(/"}},
	} {
		sn, err := snippet(s.action, s.args)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, sn)
	}
	out := runSnippets(t, p, app, mu, html_, steps)
	if !strings.Contains(out[1], "f12 [text] Nota") || !strings.Contains(out[1], "f13 [text] Nota #dn") || !strings.Contains(out[1], "· dialog") || strings.Contains(out[1], "Nombre") {
		t.Errorf("find lists the two Nota fields with their refs, the dialog's marked, and nothing else: %q", out[1])
	}
	if !strings.Contains(out[2], "[button] Guardar") || strings.Contains(out[2], "Cancelar") {
		t.Errorf("a regex finds the button: %q", out[2])
	}
	if !strings.Contains(out[3], "No field, button or text matches") || !strings.HasPrefix(out[4], "Failed: find needs text or regex") {
		t.Errorf("no match and no query are said: %q, %q", out[3], out[4])
	}
	if !strings.Contains(out[5], "f13: ok") {
		t.Errorf("fill takes a ref as the name: %q", out[5])
	}
	if !strings.Contains(out[6], "f13 [text] Nota #dn = \"alfa\"") || strings.Contains(out[6], "f12") {
		t.Errorf("the field the ref named is the one that heard it: %q", out[6])
	}
	if !strings.Contains(out[7], "clicked") || !strings.Contains(out[7], "/api/orders") {
		t.Errorf("click takes a ref: %q", out[7])
	}
	if !strings.HasPrefix(out[8], "Failed: no button b999") {
		t.Errorf("a ref past the last button is refused: %q", out[8])
	}
	if !strings.Contains(out[10], `Origen #mi = "C250" shows "PAIS 250"`) {
		t.Errorf("a menu field is found by the option text it draws, not only by its code: %q", out[10])
	}
	if !strings.Contains(out[11], "css:[name=c]: ok") || !strings.Contains(out[12], `Código #c = "por css"`) {
		t.Errorf("fill takes a CSS selector for a field with no label to name: %q, %q", out[11], out[12])
	}
	if !strings.Contains(out[13], "css:[name=nada]: not found") || !strings.Contains(out[13], "css:((: not found") {
		t.Errorf("a selector that matches no field, or is not one, is not found: %q", out[13])
	}
	if !strings.Contains(out[14], "[button] Guardar") || !strings.Contains(out[14], "[button] Cancelar") {
		t.Errorf("a text written as /A|B/ is a regex: %q", out[14])
	}
	if !strings.Contains(out[15], "No field, button or text matches") || !strings.Contains(out[15], `find {"regex":"A|B"}`) {
		t.Errorf("a text with | that finds nothing says it was searched as written: %q", out[15])
	}
	if !strings.HasPrefix(out[16], "Failed: bad regex /(/") {
		t.Errorf("a bad regex written as /…/ is said: %q", out[16])
	}
}
