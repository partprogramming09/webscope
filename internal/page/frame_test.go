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

// framePage is a layout that breaks only on a narrow window: below 550 px a text pushes two buttons out of a box that clips.
// The steps run in the top page only: a frame of the same page must not run them again.
const framePage = `<!doctype html><meta charset=utf-8><title>Responsive</title>
<style>#row{display:flex;overflow:hidden;width:100%} #pad{flex:none;width:50px} #row button{flex:none;width:80px} @media (max-width:550px){#pad{width:370px}}</style>
<h1>Pedido</h1><div id=row><span id=pad>Texto</span><button>Uno</button><button>Dos</button></div>
<pre id=out></pre>
<script type=application/json id=steps>{{STEPS}}</script>
<script>
if(top===window){
const pend=[],_ap=document.head.appendChild.bind(document.head);document.head.appendChild=e=>{if(e.tagName==='SCRIPT')pend.push(new Promise(r=>{e.addEventListener('load',r);e.addEventListener('error',r)}));return _ap(e)}
const steps=JSON.parse(document.getElementById('steps').textContent),res=[]
const sleep=ms=>new Promise(r=>setTimeout(r,ms))
;(async()=>{for(const s of steps){try{res.push(String(await (0,eval)("(async()=>("+s+"))()")))}catch(e){res.push('THROW '+e)}await Promise.all(pend.splice(0));await sleep(300)}out.textContent=JSON.stringify(res)})()
}
</script>`

// runFrameSteps serves framePage (the path /deny sends X-Frame-Options: DENY) and runs the steps in a real Chrome, with flags added.
func runFrameSteps(t *testing.T, path string, flags []string, steps func(load string) []string) []string {
	t.Helper()
	var mu sync.Mutex
	var html_ string
	app := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/deny" {
			w.Header().Set("X-Frame-Options", "DENY")
		}
		io.WriteString(w, html_)
	}))
	app.Start()
	t.Cleanup(app.Close)
	p := testPage(t, app.URL)
	load, err := askPage(t, p, "load", nil)
	if err != nil {
		t.Fatal(err)
	}
	list := steps(load)
	raw, _ := json.Marshal(list)
	mu.Lock()
	html_ = strings.Replace(framePage, "{{STEPS}}", string(raw), 1)
	mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	all := append([]string{"--headless=new", "--disable-gpu", "--no-first-run", "--no-default-browser-check", "--user-data-dir=" + t.TempDir(), "--virtual-time-budget=60000", "--dump-dom"}, flags...)
	if os.Getenv("CI") != "" {
		all = append(all, "--no-sandbox")
	}
	dom, err := exec.CommandContext(ctx, chromePath(t), append(all, app.URL+path)...).Output()
	if err != nil {
		t.Fatalf("chrome: %v", err)
	}
	m := regexp.MustCompile(`(?s)<pre id="out">(.*?)</pre>`).FindSubmatch(dom)
	if m == nil {
		t.Fatalf("the page did not finish its steps; DOM tail: %.600s", dom[max(0, len(dom)-600):])
	}
	var out []string
	if err := json.Unmarshal([]byte(html.UnescapeString(string(m[1]))), &out); err != nil || len(out) != len(list) {
		t.Fatalf("results %q: %v", m[1], err)
	}
	return out
}

func TestFrameAuditAtAWidthGivesWhatAWindowOfThatWidthGives(t *testing.T) {
	audit := "window.__ws.audit({})"
	wide := runFrameSteps(t, "/", nil, func(load string) []string {
		return []string{load, audit, "window.__ws.frame({width:360})", "String(!!document.getElementById('__ws_frame'))"}
	})
	if !strings.Contains(wide[1], "No findings") {
		t.Fatalf("the layout must be healthy in a wide window: %s", wide[1])
	}
	inFrame := wide[2]
	head, body, _ := strings.Cut(inFrame, "\n")
	if !strings.Contains(head, "frame 360px") {
		t.Errorf("the answer must say it ran in a frame of that width: %q", inFrame)
	}
	for _, want := range []string{"clipped (2)", "Uno", "Dos", "overflow (1)", "#row"} {
		if !strings.Contains(body, want) {
			t.Errorf("the frame at 360 lacks %q: %s", want, inFrame)
		}
	}
	if wide[3] != "false" {
		t.Errorf("the frame must be gone after the run unless keep is set: %s", wide[3])
	}
	// Chrome will not open a window narrower than 500 px, so the comparison is at 500: frame and window must give the same findings
	sideBySide := runFrameSteps(t, "/", nil, func(load string) []string {
		return []string{load, "window.__ws.frame({width:500})"}
	})
	_, inFrame500, _ := strings.Cut(sideBySide[1], "\n")
	narrow := runFrameSteps(t, "/", []string{"--window-size=500,700"}, func(load string) []string {
		return []string{load, "String(innerWidth)", audit}
	})
	if narrow[1] != "500" {
		t.Fatalf("the window must be 500 px wide to compare with the frame, it is %s", narrow[1])
	}
	if !strings.Contains(narrow[2], "clipped") || inFrame500 != narrow[2] {
		t.Errorf("frame at 500 and a window at 500 must agree and find the clipped button:\nframe:  %s\nwindow: %s", inFrame500, narrow[2])
	}
}

func TestFrameRunsStateInsideAndKeepsTheFrameOnRequest(t *testing.T) {
	out := runFrameSteps(t, "/", nil, func(load string) []string {
		return []string{load, "window.__ws.frame({width:500, run:'state', keep:true})", "String(document.getElementById('__ws_frame').getBoundingClientRect().width)", "window.__ws.frame({close:true})", "String(!!document.getElementById('__ws_frame'))"}
	})
	if !strings.Contains(out[1], "frame 500px") || !strings.Contains(out[1], "Pedido") {
		t.Errorf("state inside the frame must show the screen: %s", out[1])
	}
	if out[2] != "500" || out[4] != "false" {
		t.Errorf("keep leaves a frame of 500 px and close removes it: %q %q", out[2], out[4])
	}
}

func TestFrameFailsWithNextWhenTheAppRefusesToBeFramed(t *testing.T) {
	out := runFrameSteps(t, "/deny", nil, func(load string) []string {
		return []string{load, "window.__ws.frame({width:360})", "window.__ws.frame({width:50})", "String(!!document.getElementById('__ws_frame'))"}
	})
	if !strings.HasPrefix(out[1], "Failed:") || !strings.Contains(out[1], "Next:") || !strings.Contains(out[1], "by hand") {
		t.Errorf("a page that cannot be framed must fail with a Next: %q", out[1])
	}
	if !strings.HasPrefix(out[2], "Failed:") || !strings.Contains(out[2], "Next:") {
		t.Errorf("a width out of range must fail with a Next: %q", out[2])
	}
	if out[3] != "false" {
		t.Errorf("a failed frame leaves nothing in the page: %s", out[3])
	}
}
