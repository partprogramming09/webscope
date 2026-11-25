package page

import (
	"strings"
	"testing"
)

// html puts fixtures into the page one step before audit, so the layout is settled when audit measures it.
func fixtureStep(html string) string {
	return "(document.body.insertAdjacentHTML('beforeend'," + jsString(html) + "),'ok')"
}

func jsString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`)
	return "'" + r.Replace(s) + "'"
}

func TestAuditFindsClippedControlsAndOverflowAndLeavesHealthyLayoutsAlone(t *testing.T) {
	const dialog = `<div id=case1><div id=dlg role=dialog style="width:360px;overflow:hidden;display:flex;flex-wrap:nowrap;margin:0;padding:0"><span style="flex:none;width:370px">Texto largo</span><button style="flex:none;width:80px">Uno</button><button style="flex:none;width:80px">Dos</button><button style="flex:none;width:80px">Tres</button><button style="flex:none;width:80px">Cuatro</button></div></div>`
	const frame = `<div id=case2><div id=marco style="width:354px;overflow:hidden"><div id=doc style="width:516px">Factura de venta</div></div></div>`
	const iframe = `<div id=case3><iframe id=fr scrolling=no style="width:354px;height:80px;border:0" srcdoc="<body style='margin:0'><div style='width:516px'>Factura</div></body>"></iframe></div>`
	const healthy = `<div id=case4>
<div id=sc style="width:200px;overflow-x:auto"><div style="width:600px"><button>Uno</button><button style="margin-left:400px">Dos</button></div></div>
<div aria-hidden=true style="width:200px;overflow:hidden"><div style="width:800px;transform:translateX(-300px)"><button style="margin-left:400px">Oculto</button></div></div>
<div class=track-host style="width:200px;overflow:hidden"><div style="width:800px;transform:translateX(-300px)"><button style="margin-left:400px">Slide</button></div></div>
<div style="width:100px;overflow:hidden;white-space:nowrap;text-overflow:ellipsis">Un texto bastante mas largo que la caja</div>
<div style="width:300px;overflow:hidden"><button>Cabe</button></div>
<div style="width:200px;height:0;overflow:hidden"><button>Plegado</button></div>
</div>`
	out, _ := runSteps(t, func(load string) []string {
		a := func(sel string) string { return "window.__ws.audit({selector:'" + sel + "'})" }
		return []string{
			load,
			fixtureStep(dialog), a("#case1"),
			fixtureStep(frame), a("#case2"),
			fixtureStep(iframe), "(new Promise(r=>setTimeout(()=>r('ok'),600)))", a("#case3"),
			fixtureStep(healthy), a("#case4"),
		}
	}, nil)
	c1, c2, c3, c4 := out[2], out[4], out[7], out[9]
	for _, want := range []string{"clipped (4)", "Uno", "Cuatro", "overflow (1)", "#dlg", "690px", "360px"} {
		if !strings.Contains(c1, want) {
			t.Errorf("dialog with four cut buttons lacks %q: %s", want, c1)
		}
	}
	if strings.Contains(c1, "covered") {
		t.Errorf("a clipped control is not a covered one: %s", c1)
	}
	for _, want := range []string{"overflow (1)", "#marco", "516px", "354px"} {
		if !strings.Contains(c2, want) {
			t.Errorf("a 516px document in a 354px frame lacks %q: %s", want, c2)
		}
	}
	if strings.Contains(c2, "clipped") {
		t.Errorf("no control is cut in the frame: %s", c2)
	}
	for _, want := range []string{"overflow (1)", "#fr", "516px", "354px"} {
		if !strings.Contains(c3, want) {
			t.Errorf("a 516px document inside a 354px iframe lacks %q: %s", want, c3)
		}
	}
	if !strings.Contains(c4, "No findings") {
		t.Errorf("scroll containers, aria-hidden or transformed carousels, ellipsis and collapsed boxes are not findings: %s", c4)
	}
}

func TestAuditReportsAPageWiderThanTheWindow(t *testing.T) {
	out, _ := runSteps(t, func(load string) []string {
		return []string{
			load,
			fixtureStep(`<div id=wide style="width:2000px;height:10px"></div>`),
			"window.__ws.audit({})",
		}
	}, nil)
	for _, want := range []string{"overflow (1)", "page 20", "wide in a window of"} {
		if !strings.Contains(out[2], want) {
			t.Errorf("a page wider than the window lacks %q: %s", want, out[2])
		}
	}
}
