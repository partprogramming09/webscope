package page

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/partprogramming09/webscope/internal/input"
)

const (
	maxShapeBody  = 64 << 10 // a shape bigger than this is refused (413)
	maxShapeNames = 20       // shapes kept; saving one more drops nothing, it is refused
	maxNameLen    = 40
)

// field is one input of a screen: its label and its kind (text, select-one, checkbox, ...).
type field struct {
	L string `json:"l"`
	T string `json:"t"`
}

// Shape is what a screen offers, as page.js reports it: no values, only names.
type Shape struct {
	Fields   []field  `json:"fields"`
	Buttons  []string `json:"buttons"`
	Columns  []string `json:"columns"`
	Tabs     []string `json:"tabs"`
	Headings []string `json:"headings"`
}

// shape stores the shape the page reports under the name in the query.
func (p *Tool) shape(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if r.Method != http.MethodPost || name == "" || len(name) > maxNameLen {
		http.Error(w, "POST with a name of 1 to 40 characters", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxShapeBody))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		return
	}
	var s Shape
	if err != nil || json.Unmarshal(body, &s) != nil {
		http.Error(w, "bad shape", http.StatusBadRequest)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.shapes[name]; !ok && len(p.shapes) >= maxShapeNames {
		http.Error(w, "too many shapes", http.StatusConflict)
		return
	}
	if p.shapes == nil {
		p.shapes = map[string]Shape{}
	}
	p.shapes[name] = s
	w.WriteHeader(http.StatusNoContent)
}

// diff compares two lists of names ignoring case and spaces: what only a has, and what only b has.
func diff(a, b []string) (onlyA, onlyB []string) {
	key := func(s string) string {
		return strings.TrimSuffix(strings.ToLower(strings.Join(strings.Fields(s), " ")), "[]")
	}
	in := func(list []string) map[string]bool {
		m := map[string]bool{}
		for _, s := range list {
			m[key(s)] = true
		}
		return m
	}
	ma, mb := in(a), in(b)
	for _, s := range a {
		if !mb[key(s)] {
			onlyA = append(onlyA, s)
		}
	}
	for _, s := range b {
		if !ma[key(s)] {
			onlyB = append(onlyB, s)
		}
	}
	return
}

// collapse lists each name once, with ×N when it repeats.
func collapse(kind string, names []string) []string {
	var order []string
	n := map[string]int{}
	for _, s := range names {
		s = clip(oneLine(s), 60)
		if n[s] == 0 {
			order = append(order, s)
		}
		n[s]++
	}
	for i, s := range order {
		order[i] = kind + " " + s
		if n[s] > 1 {
			order[i] += fmt.Sprintf(" ×%d", n[s])
		}
	}
	return order
}

// parity shows only what a screen has and the other lacks (fields, buttons, columns, tabs) and the fields that kept their label but changed kind.
func (p *Tool) parity(args map[string]any) (string, error) {
	if input.Str(args, "req_a") != "" || input.Str(args, "req_b") != "" {
		return p.parityRequests(args)
	}
	an, bn := input.Str(args, "a"), input.Str(args, "b")
	if an == "" || bn == "" {
		return "", errors.New("parity needs a and b, the names of two saved shapes")
	}
	p.mu.Lock()
	a, okA := p.shapes[an]
	b, okB := p.shapes[bn]
	p.mu.Unlock()
	for _, m := range []struct {
		name string
		ok   bool
	}{{an, okA}, {bn, okB}} {
		if !m.ok {
			return "", fmt.Errorf("no shape named %q: open that screen and run page(shape, {name: %q})", m.name, m.name)
		}
	}
	labels := func(s Shape) []string {
		var l []string
		for _, f := range s.Fields {
			l = append(l, f.L)
		}
		return l
	}
	kinds := func(s Shape) map[string]string {
		m := map[string]string{}
		for _, f := range s.Fields {
			m[strings.ToLower(strings.Join(strings.Fields(f.L), " "))] = f.T
		}
		return m
	}
	want := map[string]bool{}
	for _, k := range strings.FieldsFunc(input.Str(args, "kinds"), func(r rune) bool { return r == ',' || r == ' ' }) {
		if !slices.Contains([]string{"field", "button", "column", "tab", "heading"}, k) {
			return "", fmt.Errorf("unknown kind %q: kinds is a list of field, button, column, tab, heading", k)
		}
		want[k] = true
	}
	var only [2][]string
	var changes []string
	for _, k := range []struct {
		kind string
		a, b []string
	}{{"field", labels(a), labels(b)}, {"button", a.Buttons, b.Buttons}, {"column", a.Columns, b.Columns}, {"tab", a.Tabs, b.Tabs}, {"heading", a.Headings, b.Headings}} {
		if len(want) > 0 && !want[k.kind] {
			continue
		}
		x, y := diff(k.a, k.b)
		only[0] = append(only[0], collapse(k.kind, x)...)
		only[1] = append(only[1], collapse(k.kind, y)...)
	}
	ka, kb := kinds(a), kinds(b)
	for _, f := range a.Fields {
		if len(want) > 0 && !want["field"] {
			break
		}
		key := strings.ToLower(strings.Join(strings.Fields(f.L), " "))
		if t, ok := kb[key]; ok && t != ka[key] {
			changes = append(changes, fmt.Sprintf("field %s: %s → %s", clip(oneLine(f.L), 60), clip(ka[key], 20), clip(t, 20)))
		}
	}
	if len(only[0])+len(only[1])+len(changes) == 0 {
		return fmt.Sprintf("%s and %s: no differences in fields, buttons, columns, tabs or headings (names only; a hint, not proof the screens behave the same)", clip(an, 40), clip(bn, 40)), nil
	}
	out := []string{fmt.Sprintf("%s → %s: only what differs, by name (case and spaces ignored)", clip(an, 40), clip(bn, 40))}
	for i, n := range []string{an, bn} {
		if len(only[i]) > 0 {
			out = append(out, "only in "+clip(n, 40)+":\n  "+strings.Join(only[i], "\n  "))
		}
	}
	if len(changes) > 0 {
		out = append(out, "changes:\n  "+strings.Join(changes, "\n  "))
	}
	s := strings.Join(out, "\n")
	if len(s) > input.MaxAnswer {
		s = clip(s, input.MaxAnswer)
	}
	return s, nil
}
