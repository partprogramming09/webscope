package code

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/partprogramming09/webscope/internal/fsx"
	"github.com/partprogramming09/webscope/internal/input"
)

var endpointRe = regexp.MustCompile(`(?i)^\s*(get|post|put|patch|delete)\s+(?:https?://[^/\s]+)?/?(\S*)\s*$`)

// contract compares what the front calls to an endpoint send with the validation rules of its backend method: only the differences.
func (c *Tool) contract(args map[string]any) (string, error) {
	m := endpointRe.FindStringSubmatch(input.Str(args, "endpoint"))
	if m == nil {
		return "", errors.New("contract needs endpoint as METHOD /path, such as POST /api/orders")
	}
	method := strings.ToUpper(m[1])
	path, _, _ := strings.Cut(m[2], "?")
	path = strings.TrimPrefix(strings.Trim(path, "/"), "api/")
	if c.Backend == "" {
		return "", errors.New("WEBSCOPE_BACKEND is not set")
	}
	files, root, err := fsx.PHPFiles(c.Backend)
	if err != nil {
		return "", err
	}
	routes := fileRoutes(files, fsx.Lines{})
	route, ok := pickRoute(routes, method, path)
	if !ok {
		return "", fmt.Errorf("no route %s %s in the Routes/*.php of the backend; map_back {query: %q} lists the routes near it", method, echo(path), path)
	}
	cls, fn, _ := strings.Cut(route.action, "@")
	short := cls[strings.LastIndex(cls, `\`)+1:]
	out := []string{fmt.Sprintf("contract %s %s -> %s@%s", route.method, route.uri, short, fn)}
	ctl := fileOf(root, cls)
	if ctl == "" {
		out[0] += "  (controller file not found)"
		return strings.Join(out, "\n"), nil
	}
	out[0] += fmt.Sprintf("  %s:%d", relTo(root, ctl), functionLine(ctl, fn))
	rs, err := rulesFor(root, ctl, fn)
	if err != nil {
		return "", err
	}
	switch {
	case rs.source == "":
		out = append(out, "rules: none (no FormRequest or validate in the method)")
	case rs.file == "":
		out = append(out, fmt.Sprintf("rules %s (file not found: its use line or namespace leads to no file under the backend; the rules are unknown)", rs.source))
	default:
		req := 0
		for _, r := range rs.rules {
			if r.required {
				req++
			}
		}
		out = append(out, fmt.Sprintf("rules %s  %s: %d keys, %d required", rs.source, relTo(root, rs.file), len(rs.rules), req))
		if !rs.literal {
			out = append(out, "incomplete: the rules are not a plain [ … ] (array_merge, parent::rules(), +): keys may be missing, so «sent, not in rules» is left out")
		}
	}
	return capOutHint(strings.Join(append(out, c.frontSide(routes, route, rs)...), "\n"), "the front calls that did not fit are left out"), nil
}

// pickRoute is the route with this method whose address fits path; a literal segment beats a parameter, and PATCH also takes a PUT route.
func pickRoute(routes []backRoute, method, path string) (backRoute, bool) {
	var fit []backRoute
	for _, r := range routes {
		if (r.method == method || method == "PATCH" && r.method == "PUT") && uriFits(r.uri, path) {
			fit = append(fit, r)
		}
	}
	if len(fit) == 0 {
		return backRoute{}, false
	}
	sort.SliceStable(fit, func(i, j int) bool {
		if a, b := strings.Count(fit[i].uri, "{"), strings.Count(fit[j].uri, "{"); a != b {
			return a < b
		}
		return fit[i].method == method && fit[j].method != method
	})
	return fit[0], true
}

// functionLine is the line of `function name(` in file, or 0.
func functionLine(file, name string) int {
	raw, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	text := blankComments(string(raw), true)
	if loc := regexp.MustCompile(`\bfunction\s+` + regexp.QuoteMeta(name) + `\s*\(`).FindStringIndex(text); loc != nil {
		return lineAt(text, loc[0])
	}
	return 0
}

// frontSide is one block per front call that the route answers: what it sends and how that differs from the rules.
func (c *Tool) frontSide(routes []backRoute, route backRoute, rs ruleSet) []string {
	if c.Front == "" {
		return []string{"front: WEBSCOPE_FRONT is not set"}
	}
	base, froot, err := c.frontBase()
	if err != nil {
		return []string{"front: " + err.Error()}
	}
	mods, names, err := indexModules(base)
	if err != nil {
		return []string{"front: " + err.Error()}
	}
	snake := caseConverter(froot)
	types := &typeIndex{root: froot} // types live anywhere in the front (src/types too), read once for every call
	how := "(names compared as written)"
	if snake {
		how = "(camelCase -> snake_case: axios-case-converter)"
	}
	var out []string
	for _, n := range names {
		for _, call := range mods[n].calls {
			head, where, _ := strings.Cut(call, "  ")
			cm, url, _ := strings.Cut(head, " ")
			if r, ok := pickRoute(routes, cm, callKey(url)); !ok || r != route { // the route that would answer this call
				continue
			}
			file, ln := splitWhere(where)
			path := filepath.Join(base, file)
			if !strings.HasPrefix(path, base+string(filepath.Separator)) {
				continue
			}
			if cm == "GET" || cm == "DELETE" {
				out = append(out, fmt.Sprintf("front %s:%d %s has no body: nothing compared", relTo(froot, path), ln, cm))
				continue
			}
			fields, unresolved := sentFieldsIn(types, path, ln)
			for i := range fields {
				fields[i].name = wireName(fields[i].name, snake)
			}
			fields = mergeSent(fields)
			read := ""
			if len(unresolved) > 0 {
				read = " (read incomplete)"
			}
			out = append(out, fmt.Sprintf("front %s:%d sends %d%s %s", relTo(froot, path), ln, len(fields), read, how))
			out = append(out, gaps(fields, rs, len(unresolved) == 0)...)
			if len(unresolved) > 0 {
				out = append(out, "incomplete: "+strings.Join(unresolved, "; "))
			}
		}
	}
	if len(out) == 0 {
		return []string{"front: no call to this endpoint in WEBSCOPE_FRONT"}
	}
	return out
}

// splitWhere splits "file:line".
func splitWhere(where string) (string, int) {
	i := strings.LastIndex(where, ":")
	if i < 0 {
		return where, 0
	}
	n := 0
	fmt.Sscanf(where[i+1:], "%d", &n)
	return where[:i], n
}

// gaps lists the differences between the fields a call sends and the rules: each section only when it has names.
// complete is false when part of the body could not be read: then a required rule is only «not in what could be read».
func gaps(fields []sent, rs ruleSet, complete bool) []string {
	if rs.source == "" {
		return nil
	}
	rules := map[string]rule{}
	for _, r := range rs.rules {
		rules[r.name] = r
	}
	sentBy := map[string]sent{}
	for _, f := range fields {
		sentBy[f.name] = f
	}
	var extra, missing, notSent, optional []string
	for _, f := range fields {
		r, ok := rules[f.name]
		switch {
		case !ok && rs.literal:
			extra = append(extra, f.name)
		case ok && r.required && f.optional:
			optional = append(optional, f.name)
		}
	}
	for _, r := range rs.rules {
		if _, ok := sentBy[r.name]; !ok {
			notSent = append(notSent, r.name)
			if r.required {
				missing = append(missing, r.name)
			}
		}
	}
	var near []string
	for _, s := range extra {
		for _, r := range notSent {
			if nearName(s, r) {
				near = append(near, "sent "+s+" ~ rule "+r)
			}
		}
	}
	var out []string
	add := func(label string, names []string) {
		if len(names) > 0 {
			sort.Strings(names)
			out = append(out, label+strings.Join(names, ", "))
		}
	}
	add("sent, not in rules (dropped if the method reads validated()): ", extra)
	if complete {
		add("required, not sent: ", missing)
	} else {
		add("required, not in what could be read: ", missing)
	}
	add("near names: ", near)
	add("required in rules, optional in the front type: ", optional)
	return out
}

// nearName is true when a and b are the same once _ and an _id suffix are dropped, or differ by 2 edits at most with 5 letters or more.
func nearName(a, b string) bool {
	norm := func(s string) string {
		return strings.ReplaceAll(strings.TrimSuffix(strings.ToLower(s), "_id"), "_", "")
	}
	if norm(a) == norm(b) {
		return true
	}
	return len(a) >= 5 && len(b) >= 5 && editDistance(a, b) <= 2
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
