package page

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const (
	maxKeys  = 200 // names kept from one body
	maxDepth = 4
)

// last is the latest capture whose method and address match spec ("POST /api/quotes": the method, then a part of the address).
func (p *Tool) last(spec string) (Capture, bool) {
	method, part, _ := strings.Cut(strings.TrimSpace(spec), " ")
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := len(p.caps) - 1; i >= 0; i-- {
		if c := p.caps[i]; strings.EqualFold(c.M, method) && strings.Contains(c.U, strings.TrimSpace(part)) {
			return c, true
		}
	}
	return Capture{}, false
}

// walk adds the names of v to out: objects by key, nested ones with a dot, arrays by their first element with [].
func walk(v any, prefix string, depth int, out map[string]bool) {
	if len(out) >= maxKeys || depth > maxDepth {
		return
	}
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			name := k
			if prefix != "" {
				name = prefix + "." + k
			}
			switch c.(type) {
			case map[string]any, []any:
				walk(c, name, depth+1, out)
			default:
				out[name] = true
			}
		}
	case []any:
		if len(x) > 0 {
			walk(x[0], prefix+"[]", depth+1, out)
		}
	}
}

// names is the set of field names of a request or response body (JSON or form), or why it cannot be read. Never a value.
func names(body string) (map[string]bool, string) {
	body = strings.TrimSpace(body)
	out := map[string]bool{}
	if body == "" {
		return out, ""
	}
	if strings.Contains(body, "…(+") {
		return nil, "cut"
	}
	var v any
	if json.Unmarshal([]byte(body), &v) == nil {
		walk(v, "", 0, out)
		return out, ""
	}
	if q, err := url.ParseQuery(body); err == nil && strings.Contains(body, "=") {
		for k := range q {
			out[k] = true
		}
		return out, ""
	}
	return nil, "not JSON or form"
}

// side lists the names only a has, sorted, after renaming a's with rename.
func side(a, b map[string]bool, rename map[string]string) (onlyA, onlyB []string) {
	ra := map[string]bool{}
	for k := range a {
		if n, ok := rename[k]; ok {
			k = n
		}
		ra[k] = true
	}
	for k := range ra {
		if !b[k] {
			onlyA = append(onlyA, k)
		}
	}
	for k := range b {
		if !ra[k] {
			onlyB = append(onlyB, k)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return
}

// parityRequests compares the latest captured request that matches each spec: status and the field names sent and received, never values.
func (p *Tool) parityRequests(args map[string]any) (string, error) {
	var caps [2]Capture
	for i, k := range []string{"req_a", "req_b"} {
		spec, _ := args[k].(string)
		c, ok := p.last(spec)
		if !ok {
			return "", fmt.Errorf("no captured request matches %q (%s): run the action on the screen and page(payloads) to see what was captured", clip(oneLine(spec), 60), k)
		}
		caps[i] = c
	}
	rename := map[string]string{}
	if m, ok := args["map"].(map[string]any); ok {
		for k, v := range m {
			if s, ok := v.(string); ok {
				rename[k] = s
			}
		}
	}
	var queries [2]map[string]bool
	line := func(i int) string {
		c := caps[i]
		path, q := target(c.U)
		queries[i] = q
		where := clip(oneLine(path), 120)
		if len(q) > 0 {
			where += fmt.Sprintf(" ?%d params", len(q))
		}
		return clip(oneLine(c.M), 10) + " " + where + " " + fmt.Sprint(c.S)
	}
	out := []string{"a: " + line(0) + " · b: " + line(1) + " (field names only, never values)"}
	if caps[0].S != caps[1].S {
		out = append(out, fmt.Sprintf("status: %d → %d", caps[0].S, caps[1].S))
	}
	same := true
	inA, inB := map[string]bool{}, map[string]bool{} // the names each side has, in what was sent and in what was got
	for _, part := range []struct{ label, a, b string }{{"sent", caps[0].Req, caps[1].Req}, {"got", caps[0].Res, caps[1].Res}} {
		na, ea := names(part.a)
		nb, eb := names(part.b)
		if part.label == "sent" { // a GET sends its fields in the address
			for k := range queries[0] {
				if na != nil {
					na[k] = true
				}
			}
			for k := range queries[1] {
				if nb != nil {
					nb[k] = true
				}
			}
		}
		for k := range na {
			inA[k] = true
		}
		for k := range nb {
			inB[k] = true
		}
		if ea != "" || eb != "" {
			out = append(out, fmt.Sprintf("%s: not compared (a: %s, b: %s)", part.label, firstNonEmpty(ea, "ok"), firstNonEmpty(eb, "ok")))
			same = false
			continue
		}
		oa, ob := side(na, nb, rename)
		if len(oa) > 0 {
			out = append(out, part.label+" only in a: "+strings.Join(oa, ", "))
		}
		if len(ob) > 0 {
			out = append(out, part.label+" only in b: "+strings.Join(ob, ", "))
		}
		same = same && len(oa)+len(ob) == 0
	}
	if len(rename) > 0 { // a rename that found no name does nothing, and the answer would not show it
		var bad []string
		matched := 0
		for _, k := range slices.Sorted(maps.Keys(rename)) {
			switch {
			case !inA[k]:
				bad = append(bad, "«"+clip(oneLine(k), 40)+"»: not in a")
			case !inB[rename[k]]:
				bad = append(bad, "«"+clip(oneLine(k), 40)+"» → «"+clip(oneLine(rename[k]), 40)+"»: not in b")
			default:
				matched++
			}
		}
		out = append(out, fmt.Sprintf("map: %d of %d names matched", matched, len(rename)))
		out = append(out, bad...)
	}
	if same && caps[0].S == caps[1].S {
		out = append(out, "same status and same field names sent and received")
	}
	return clip(strings.Join(out, "\n"), 8000), nil
}

var listIndex = regexp.MustCompile(`\[\d+\]`)

// target is the address of a request without its query string, and the names of the query parameters (a list index is written []).
func target(raw string) (string, map[string]bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.SplitN(raw, "?", 2)[0], nil
	}
	names := map[string]bool{}
	for k := range u.Query() {
		names[listIndex.ReplaceAllString(k, "[]")] = true
	}
	return u.Host + u.Path, names
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}
