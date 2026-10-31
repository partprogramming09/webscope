// Package page is the page tool: snippets for testing a screen in Chrome, and the local listener that keeps the requests the page sends.
package page

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/partprogramming09/webscope/internal/input"
)

//go:embed page.js
var pageJS string

// pageVer identifies the embedded helper: the first 8 hex digits of its hash.
var pageVer = fmt.Sprintf("%x", sha256.Sum256([]byte(pageJS)))[:8]

const (
	maxCaptureBody = 256 << 10 // a capture bigger than this is refused (413)
	maxCaptures    = 50        // requests kept; the oldest go first
	staleAfter     = 30        // seconds without a capture after which payloads says how to reload the helper
	storeClip      = 16 << 10  // bytes of each body kept (check compares them whole; a list answer of Acme passed 2000)
	showClip       = 600       // bytes of each body shown by payloads
	defaultShown   = 10        // requests shown by payloads unless limit says otherwise
)

// snippetActions are the actions answered with a JavaScript snippet for javascript_tool. Anything else is answered here.
var snippetActions = []string{"state", "read", "fill", "click", "press", "simulate", "open", "options", "audit", "shape"}

const helpText = `page(action, args?) tests a screen in the user's Chrome through Claude in Chrome.
Needs a tab of the app (WEBSCOPE_APP_URL) with the session already started: open one with tabs_create_mcp and navigate to a screen; if it shows the login, stop and ask the user for the session (never type credentials).
load: one line of JavaScript; run it with javascript_tool again after every navigation, reload or /mcp: the helper is lost and the port changes (needs WEBSCOPE_APP_URL).
state|read|fill|click|press|simulate|open|options|audit|shape: return a snippet; run it with javascript_tool, its value is the answer.
  options {field, match?, max?, fresh?}: the choices of a select or Vuetify menu, without choosing one: all of a native select, and of a menu the rows seen while it scrolls (an autocomplete is searched with match); only those that contain match, up to max (30). A menu read to its end is remembered 2 minutes (the answer says how old it is): fresh:true reads it again.
  open {path}: goes to a router path from code map (/pricing/sales-tariffs) through the app's own router, which adds the tenant prefix; a path that no route owns is refused with the similar ones, and a redirect (login) is told. Never type the URL of a screen by hand.
  state {max?}: URL, title, console, headings, fields, buttons (dialog first, side menu last, disabled ones marked), alerts, errors, dialog, tables.
  read {selector?, max?}: text of the first match, or of the page.
  fill {fields:{label:value}} or {label:value}: by label, name, placeholder or id (a name that fits several fields fills the first, a dialog's before the page's, and the answer warns and lists the others: use an id); true/false for checkboxes; select by option text; Vuetify menus too (long lists are scrolled); a rich-text editor (contenteditable) by its aria-label or id, or as "editor" when it has neither.
  click {text} or {selector} (a text that belongs to no button is found as a plain element); press {key, selector?}: both then say what happened (requests with status, new alerts, errors and console errors, URL change, or "No effect seen"; with WEBSCOPE_LOG set, a status 0 or 5xx adds the last error lines of the backend log).
  audit {selector?}: what the page shows about itself, no reference screen: unrendered text ({{x}}, undefined, null, NaN, [object Object]), controls covered by another element, fields with no label, repeated labels, buttons with no name. A hint to look at, not a verdict; selector limits it to a part of the screen.
  shape {name, selector?}: saves what the screen, or only the part a CSS selector names, offers under a name (field labels and kinds, buttons, table columns, tabs, headings; no values) for parity.
  simulate {match, status, times?, method?}: the next requests whose URL contains match answer HTTP status 400-599 without reaching the app; {clear:true} cancels.
payloads {limit?}: the requests the page sent and their answers (answered here, no snippet).
parity {a, b, kinds?}: compares two saved shapes (say the old and the new screen) and shows only what one has and the other lacks (a repeated name once, with ×N), and fields whose kind changed (answered here, no snippet); kinds "field,column" limits it, e.g. to leave out the step buttons of a wizard. parity {req_a, req_b, map?} compares instead the latest captured request that matches each ("POST save.php": the method, then a part of the address): status and the field names sent and received, never values; map {name in a: name in b} renames a's names first.
download {arm:true} before a real click, then {wait:true, seconds?} after it, and {report:true} lists the files the session saw appear so the user can delete them (answered here, no snippet): waits for a new file in the downloads folder (WEBSCOPE_DOWNLOADS, else ~/Downloads) and gives its name, size, type by its first bytes and time, and for an xlsx its header names and row count, never a value. No file in time is a failure: an export counts as downloaded only if this finds the file.
verify {claim, evidence?}: adds a claim of the final report to a table and answers its state, which the server decides: OBSERVADO (evidence «capture METHOD address» of a request that was captured, «check METHOD address» of a check with no differences, or «download» of a file that download wait found), INFERIDO (that capture was seen after an interceptor was put on the page) or NO VERIFICADO (no evidence, or none the server can find). verify {report:true} prints the table, {clear:true} empties it; build the report from it (answered here, no snippet).
Writes only with the user's approval, and rows prefixed ZZ_.`

// Capture is one request the page sent, as reported by page.js.
type Capture struct {
	M   string `json:"m"`
	U   string `json:"u"`
	S   int    `json:"s"`
	O   bool   `json:"o,omitempty"` // an opaque answer (a no-cors request to another origin): its status is 0 but it did answer
	Req string `json:"req"`
	Res string `json:"res"`
}

// Tool hands out snippets and runs the local listener that serves page.js and keeps what the page captures. Life ends the listener.
type Tool struct {
	appURL string
	token  string
	Life   context.Context

	mu      sync.Mutex
	base    string
	caps    []Capture
	lastCap time.Time        // when the last capture came
	now     func() time.Time // the clock; a test sets it
	shapes  map[string]Shape

	tampered []string // page functions replaced after the helper loaded, as the helper last said

	logFrom      int64                // where the backend log ended when the last click or key began
	logMarked    bool                 // logFrom was set
	LogPath      string               // the backend log; empty means WEBSCOPE_LOG
	DownloadDir  string               // where Chrome saves files; empty means WEBSCOPE_DOWNLOADS, then ~/Downloads
	dlSeen       map[string]fileMark  // the files of that folder when download was armed; nil if not armed
	checks       map[string]checkNote // what check found, by the request it compared
	claims       []claim              // the claims of the report, with the state the server gave each
	tamperedEver bool                 // an interceptor was seen since the last page(load)

	dlFiles []string // the files that download wait reported this session, with their size
	dlLast  string   // the name of the file the last download wait reported, empty if none

	refused bool // a request that held the token came from another origin than the app's
}

func New(appURL string) *Tool {
	b := make([]byte, 16)
	rand.Read(b)
	return &Tool{appURL: appURL, token: hex.EncodeToString(b), Life: context.Background(), now: time.Now}
}

// origins are the scheme://host of each address in WEBSCOPE_APP_URL (several are separated by commas): the only origins the listener answers.
func (p *Tool) origins() []string {
	var out []string
	for _, a := range strings.Split(p.appURL, ",") {
		if o := originOf(strings.TrimSpace(a)); o != "" && !slices.Contains(out, o) {
			out = append(out, o)
		}
	}
	return out
}

// originOf is the origin of one address, as the browser writes it, or "" if it is not an address.
func originOf(appURL string) string {
	u, err := url.Parse(appURL)
	if err != nil || u.Host == "" {
		return ""
	}
	// As the browser writes the Origin header: lowercase, and without the port that is the default of the scheme.
	scheme, host, port := strings.ToLower(u.Scheme), strings.ToLower(u.Hostname()), u.Port()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port == "" || (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		return scheme + "://" + host
	}
	return scheme + "://" + host + ":" + port
}

// snippet is the JavaScript for action. The arguments enter as JSON (json.Marshal escapes <, > and line separators), never as text.
func snippet(action string, args map[string]any) (string, error) {
	if !slices.Contains(snippetActions, action) {
		return "", fmt.Errorf("unknown action %q (load, state, read, fill, click, press, payloads, simulate, audit, shape, open, options, parity, download, verify or help)", action)
	}
	if args == nil {
		args = map[string]any{}
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`await (window.__ws?window.__ws.%s(%s):"Failed: webscope is not loaded in this page. Next: page(load) and run it.")`, action, raw), nil // await: fill waits for menus
}

func (p *Tool) answer(in input.Args) (string, error) {
	switch in.Action {
	case "help":
		return helpText, nil
	case "payloads":
		return p.payloads(in.Args), nil
	case "download":
		return p.download(in.Args)
	case "verify":
		return p.verify(in.Args)
	case "parity":
		return p.parity(in.Args)
	case "load":
		if len(p.origins()) == 0 {
			return "", errors.New("WEBSCOPE_APP_URL is not set, set it to the app's address, e.g. http://localhost:5173 (several, separated by commas, when the old and the new app are on different ports)")
		}
		base, err := p.start()
		if err != nil {
			return "", err
		}
		p.mu.Lock()
		p.tamperedEver = false
		p.mu.Unlock()
		return fmt.Sprintf(`(()=>{if(window.__ws&&window.__wsVer!==%q)return "Failed: an older helper is in this page: reload the page, then page(load)";var s=document.createElement("script");s.src=%q;document.head.appendChild(s);return "loading the webscope helper; next: page(state)"})()`, pageVer, base+"/page.js?t="+p.token+"&v="+pageVer+"&o="+url.QueryEscape(strings.Join(p.origins(), ","))), nil
	}
	return snippet(in.Action, in.Args)
}

// Handle answers a page call: text on success, and on failure a text that starts with "Failed:" and names the next step, marked as an error.
func (p *Tool) Handle(_ context.Context, _ *mcp.CallToolRequest, in input.Args) (*mcp.CallToolResult, any, error) {
	text, err := p.answer(in)
	if err != nil {
		text = "Failed: " + strings.TrimRight(err.Error(), ". ") + ". Next: page(help) lists the actions and their arguments."
	}
	return &mcp.CallToolResult{IsError: err != nil, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}

// start opens the listener on 127.0.0.1 the first time it is needed and returns its address.
func (p *Tool) start() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.base != "" {
		return p.base, nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: p.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second}
	go srv.Serve(ln)
	go func() { <-p.Life.Done(); srv.Close() }()
	p.base = "http://" + ln.Addr().String()
	return p.base, nil
}

// guard lets a request through only with the token and, if it names an origin, the app's. It answers the CORS headers and the preflight.
func (p *Tool) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(p.token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			if !slices.Contains(p.origins(), o) {
				p.mu.Lock()
				p.refused = true
				p.mu.Unlock()
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", o)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			h.Set("Access-Control-Allow-Private-Network", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (p *Tool) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/page.js", p.guard(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		io.WriteString(w, pageJS)
	}))
	mux.HandleFunc("/capture", p.guard(p.capture))
	mux.HandleFunc("/shape", p.guard(p.shape))
	mux.HandleFunc("/tamper", p.guard(p.tamper))
	mux.HandleFunc("/log", p.guard(p.logRoute))
	mux.HandleFunc("/logmark", p.guard(p.logMark))
	mux.HandleFunc("/sim", p.guard(func(w http.ResponseWriter, r *http.Request) {
		status, err := strconv.Atoi(r.URL.Query().Get("status"))
		if err != nil || status < 400 || status > 599 {
			http.Error(w, "status must be 400-599", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, `{"message":"simulated by webscope"}`)
	}))
	return mux
}

func (p *Tool) capture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCaptureBody))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		return
	}
	var c Capture
	if err != nil || json.Unmarshal(body, &c) != nil || c.U == "" {
		http.Error(w, "bad capture", http.StatusBadRequest)
		return
	}
	c.Req, c.Res = clip(c.Req, storeClip), clip(c.Res, storeClip)
	p.mu.Lock()
	p.caps = append(p.caps, c)
	p.lastCap = p.now()
	if len(p.caps) > maxCaptures {
		p.caps = p.caps[len(p.caps)-maxCaptures:]
	}
	p.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// tamper records which page functions the helper saw replaced after it loaded; only the three names it watches are kept.
func (p *Tool) tamper(w http.ResponseWriter, r *http.Request) {
	var l []string
	for _, n := range strings.Split(r.URL.Query().Get("n"), ",") {
		if n == "fetch" || n == "send" || n == "anchor click" {
			l = append(l, n)
		}
	}
	p.mu.Lock()
	p.tampered = l
	p.tamperedEver = p.tamperedEver || len(l) > 0
	p.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (p *Tool) payloads(args map[string]any) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := defaultShown
	if f, ok := args["limit"].(float64); ok && f >= 1 && f <= maxCaptures {
		n = int(f)
	}
	list := p.caps[max(0, len(p.caps)-n):]
	var b strings.Builder
	if len(p.tampered) > 0 {
		fmt.Fprintf(&b, "Interceptor: %s replaced after the helper loaded. These requests are what the page did with that patch, not what a real click did: do not report them as a check of the real flow.\n", strings.Join(p.tampered, ", "))
	}
	fmt.Fprintf(&b, "%d requests captured (showing last %d)", len(p.caps), len(list))
	if len(p.caps) > 0 {
		age := int(p.now().Sub(p.lastCap).Seconds())
		fmt.Fprintf(&b, "; last one %d s ago", age)
		if age >= staleAfter {
			b.WriteString(". If the page was reloaded since, the helper is gone and nothing new is captured: run page(load) again")
		}
	}
	b.WriteString("\n")
	if s := problems(p.caps); s != "" {
		b.WriteString(s + "\n")
	}
	if p.refused { // the origin itself is not repeated: it comes from the browser, and the agent can read the tab's address with state
		fmt.Fprintf(&b, "(requests from another origin were refused: the tab's address must be one of those in WEBSCOPE_APP_URL, %s; restart the server after changing it)\n", strings.Join(p.origins(), ", "))
	}
	failed := false
	for _, c := range list {
		failed = failed || (c.S == 0 && !c.O) || c.S >= 500
	}
	for i, c := range list {
		// The method and the URL come from the page under test: one line each, and short.
		status := strconv.Itoa(c.S)
		if c.O {
			status = "opaque (no-cors: its status cannot be read)"
		}
		e := fmt.Sprintf("%s %s %s\n", clip(oneLine(c.M), 10), clip(oneLine(c.U), 300), status)
		if c.Req != "" {
			e += fmt.Sprintf("  sent: %s\n", oneLine(clip(c.Req, showClip)))
		}
		if c.Res != "" {
			e += fmt.Sprintf("  got: %s\n", oneLine(clip(c.Res, showClip)))
		}
		if b.Len()+len(e) > input.MaxAnswer {
			fmt.Fprintf(&b, "(… %d more requests not shown: ask for fewer with limit)", len(list)-i)
			return b.String()
		}
		b.WriteString(e)
	}
	from := int64(-1)
	if p.logMarked {
		from = p.logFrom
	}
	if l := p.logLinesFrom(from); failed && l != "" {
		b.WriteString("\nBackend log (the last error lines of WEBSCOPE_LOG, as the log wrote them):\n" + l)
	}
	return strings.TrimRight(b.String(), "\n")
}

// problems summarises the captured requests that failed (4xx, 5xx, no answer) over all of them, or "" when none did.
func problems(caps []Capture) string {
	var c4, c5, c0 int
	for _, c := range caps {
		switch {
		case c.O:
		case c.S == 0:
			c0++
		case c.S >= 500:
			c5++
		case c.S >= 400:
			c4++
		}
	}
	var parts []string
	for _, x := range []struct {
		n    int
		what string
	}{{c4, "4xx"}, {c5, "5xx"}, {c0, "no answer (status 0)"}} {
		if x.n > 0 {
			parts = append(parts, fmt.Sprintf("%d with %s", x.n, x.what))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("Problems: %s of %d captured", strings.Join(parts, ", "), len(caps))
}

func oneLine(s string) string { return strings.NewReplacer("\r", "", "\n", `\n`).Replace(s) }

// clip cuts s to n bytes at a character boundary and says how much was left out.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…(+" + strconv.Itoa(len(s)-n) + " bytes)"
}

// Captures is a copy of the requests the page sent, oldest first.
func (p *Tool) Captures() []Capture {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.caps)
}
