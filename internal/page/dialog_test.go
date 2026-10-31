package page

import (
	"strings"
	"testing"
)

// TestAClickThatReopensTheSameDialogIsToldApart runs in a real Chrome: a confirm button that closes its dialog and opens another with the same text (a
// repeated confirmation) is told as such, not as a click with no effect.
func TestAClickThatReopensTheSameDialogIsToldApart(t *testing.T) {
	app, mu, html_ := fixtureApp(t)
	p := testPage(t, app.URL)
	load, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	build := `(()=>{document.body.insertAdjacentHTML('afterbegin','<div role=dialog id=cf><div>¿Eliminar?</div><p>Detalle que el segundo no lleva</p><button type=button id=yes>Sí confirmar</button></div>');document.getElementById('yes').onclick=()=>{Promise.reject(new Error('unrelated'));document.getElementById('cf').remove();document.body.insertAdjacentHTML('afterbegin','<div role=dialog id=cf2><div>¿Eliminar?</div><button type=button id=yes2>Sí confirmar</button></div>')};return 'built'})()`
	click, err := snippet("click", map[string]any{"text": "Sí confirmar"})
	if err != nil {
		t.Fatal(err)
	}
	nop, err := snippet("click", map[string]any{"text": "Inerte"})
	if err != nil {
		t.Fatal(err)
	}
	dis, err := snippet("click", map[string]any{"text": "Bloqueado"})
	if err != nil {
		t.Fatal(err)
	}
	out := runSnippets(t, p, app, mu, html_, []string{load, build, click, nop, dis})
	if !strings.Contains(out[2], "open again") || !strings.Contains(out[2], "repeated confirmation") || strings.Contains(out[2], "No effect") {
		t.Errorf("a dialog that comes back with the same text is a second confirmation: %q", out[2])
	}
	if !strings.Contains(out[3], "No effect seen") {
		t.Errorf("a click that changes nothing is still told as no effect: %q", out[3])
	}
	if !strings.Contains(out[4], "it is disabled") {
		t.Errorf("a click on a disabled button says so: %q", out[4])
	}
}
