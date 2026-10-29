// Package page is the page tool: snippets for testing a screen in Chrome, and the local listener that keeps the requests the page sends.
package page

import (
	"context"
	"crypto/rand"
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

const (
	maxCaptureBody = 256 << 10 // a capture bigger than this is refused (413)
	maxCaptures    = 50        // requests kept; the oldest go first
	storeClip      = 16 << 10  // bytes of each body kept (check compares them whole; a list answer of Acme passed 2000)
	showClip       = 600       // bytes of each body shown by payloads
	defaultShown   = 10        // requests shown by payloads unless limit says otherwise
)

// snippetActions are the actions answered with a JavaScript snippet for javascript_tool. Anything else is answered here.
var snippetActions = []string{"state", "read", "fill", "click", "press", "simulate"}

const helpText = `page(action, args?) tests a screen in the user's Chrome through Claude in Chrome.
Needs a tab of the app (WEBSCOPE_APP_URL) with the session already started: open one with tabs_create_mcp and navigate to a screen; if it shows the login, stop and ask the user for the session (never type credentials).
load: one line of JavaScript; run it with javascript_tool again after every navigation, reload or /mcp: the helper is lost and the port changes (needs WEBSCOPE_APP_URL).
state|read|fill|click|press|simulate: return a snippet; run it with javascript_tool, its value is the answer.
  state {max?}: URL, title, console, headings, fields, buttons (dialog first, side menu last, disabled ones marked), alerts, errors, dialog, tables.
  read {selector?, max?}: text of the first match, or of the page.
  fill {fields:{label:value}} or {label:value}: by label, name, placeholder or id; true/false for checkboxes; select by option text; Vuetify menus too (long lists are scrolled).
  click {text} or {selector}; press {key, selector?}: both then say what happened (requests with status, new alerts, errors and console errors, URL change, or "No effect seen").
  simulate {match, status, times?, method?}: the next requests whose URL contains match answer HTTP status 400-599 without reaching the app; {clear:true} cancels.
payloads {limit?}: the requests the page sent and their answers (answered here, no snippet).
Writes only with the user's approval, and rows prefixed ZZ_.`

// Capture is one request the page sent, as reported by page.js.
type Capture struct {
	M   string `json:"m"`
	U   string `json:"u"`
	S   int    `json:"s"`
	Req string `json:"req"`
	Res string `json:"res"`
}

// Tool hands out snippets and runs the local listener that serves page.js and keeps what the page captures. Life ends the listener.
type Tool struct {
	appURL string
	token  string
	Life   context.Context

	mu   sync.Mutex
	base string
	caps []Capture

	refused bool // a request that held the token came from another origin than the app's
}

func New(appURL string) *Tool {
	b := make([]byte, 16)
	rand.Read(b)
	return &Tool{appURL: appURL, token: hex.EncodeToString(b), Life: context.Background()}
}

// origin is scheme://host of the app: the only origin the listener answers.
func (p *Tool) origin() string {
	u, err := url.Parse(p.appURL)
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
		return "", fmt.Errorf("unknown action %q (load, state, read, fill, click, press, payloads, simulate or help)", action)
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
	case "load":
		if p.origin() == "" {
			return "", errors.New("WEBSCOPE_APP_URL is not set, set it to the app's address, e.g. http://localhost:5173")
		}
		base, err := p.start()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`(()=>{var s=document.createElement("script");s.src=%q;document.head.appendChild(s);return "loading the webscope helper; next: page(state)"})()`, base+"/page.js?t="+p.token+"&o="+url.QueryEscape(p.origin())), nil
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
			if o != p.origin() {
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
	if len(p.caps) > maxCaptures {
		p.caps = p.caps[len(p.caps)-maxCaptures:]
	}
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
	fmt.Fprintf(&b, "%d requests captured (showing last %d)\n", len(p.caps), len(list))
	if p.refused { // the origin itself is not repeated: it comes from the browser, and the agent can read the tab's address with state
		fmt.Fprintf(&b, "(requests from another origin were refused: the tab's address must be the one in WEBSCOPE_APP_URL, %s; restart the server after changing it)\n", p.origin())
	}
	for i, c := range list {
		// The method and the URL come from the page under test: one line each, and short.
		e := fmt.Sprintf("%s %s %d\n", clip(oneLine(c.M), 10), clip(oneLine(c.U), 300), c.S)
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
	return strings.TrimRight(b.String(), "\n")
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
