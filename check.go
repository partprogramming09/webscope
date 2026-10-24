package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"context"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// checker compares what the screen sent, what the database stored and what the screen read afterwards.
type checker struct {
	s *source
	p *pageTool
}

// checkSchema is written by hand: the one generated from checkArgs costs twice the bytes of tools/list. sent is a captured request
// ("POST /api/quotes"), where is {column: value} for one row, map is {sent field: column}, after is a captured read ("GET /api/quotes/1")
// and at the path of the object inside its answer ("quote", "data.0"). No "required" list: it costs bytes and run says what is missing.
var checkSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"sent": map[string]any{"type": "string"}, "table": map[string]any{"type": "string"},
		"where": map[string]any{"type": "object"}, "map": map[string]any{"type": "object"},
		"after": map[string]any{"type": "string"}, "at": map[string]any{"type": "string"}, "tenant": map[string]any{"type": "string"},
	},
}

type checkArgs struct {
	Sent   string            `json:"sent"`
	Table  string            `json:"table"`
	Where  map[string]string `json:"where"`
	Map    map[string]string `json:"map"`
	After  string            `json:"after,omitempty"`
	At     string            `json:"at,omitempty"`
	Tenant string            `json:"tenant,omitempty"`
}

// cutMark is what clip (here, in bytes) and page.js (in chars) leave at the end of a body they cut.
var cutMark = regexp.MustCompile(`…\(\+\d+ (bytes|chars)\)$`)

// find is the last captured request that matches "METHOD /path" (the path of the URL, or its end).
func (c *checker) find(spec string) (capture, error) {
	method, path, ok := strings.Cut(strings.TrimSpace(spec), " ")
	if !ok || path == "" {
		return capture{}, fmt.Errorf("%q is not METHOD /path", spec)
	}
	c.p.mu.Lock()
	defer c.p.mu.Unlock()
	for i := len(c.p.caps) - 1; i >= 0; i-- {
		k := c.p.caps[i]
		if u, err := url.Parse(k.U); err == nil && strings.EqualFold(k.M, method) && strings.HasSuffix(u.Path, path) {
			return k, nil
		}
	}
	return capture{}, fmt.Errorf("no captured request matches %q (%d captured, page(payloads) lists them)", spec, len(c.p.caps))
}

// object is the JSON object of a captured body, at a path like "quote" or "data.0" when at is not empty.
func object(body, at, what string) (map[string]any, error) {
	if cutMark.MatchString(body) {
		return nil, fmt.Errorf("the %s was cut when it was captured (%s): it cannot be compared whole", what, body[strings.LastIndex(body, "…"):])
	}
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("the %s is not JSON", what)
	}
	for _, step := range strings.FieldsFunc(at, func(r rune) bool { return r == '.' }) {
		switch x := v.(type) {
		case map[string]any:
			v = x[step]
		case []any:
			n, err := strconv.Atoi(step)
			if err != nil || n < 0 || n >= len(x) {
				return nil, fmt.Errorf("%q is not in the %s", at, what)
			}
			v = x[n]
		default:
			return nil, fmt.Errorf("%q is not in the %s", at, what)
		}
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the %s has no JSON object at %q", what, at)
	}
	return o, nil
}

// value is a JSON value as text, with empty marking null and "": the two ways a screen says "nothing".
func value(v any, present bool) (text string, empty bool) {
	switch x := v.(type) {
	case nil:
		return "null", present
	case string:
		return x, x == ""
	case json.Number:
		return x.String(), false
	case bool:
		return strconv.FormatBool(x), false
	}
	b, _ := json.Marshal(v)
	return string(b), false
}

func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

type seen struct {
	text  string
	empty bool
	here  bool
}

// sameText says whether every cell shows exactly the text of the first one.
func sameText(first string, rest []seen) bool {
	for _, k := range rest {
		if k.text != first {
			return false
		}
	}
	return true
}

// show is a value for the answer: one line, 40 characters at most.
func show(s string) string {
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}

// row reads the one row of table that where picks, only the columns asked for, in a read-only transaction.
func (c *checker) row(ctx context.Context, in checkArgs, cols []string) (map[string]*string, error) {
	db, driver, err := c.s.open(ctx, in.Tenant)
	if err != nil {
		return nil, err
	}
	var conds []string
	var vals []any
	for _, k := range slices.Sorted(maps.Keys(in.Where)) {
		conds = append(conds, "`"+k+"` = ?")
		vals = append(vals, in.Where[k])
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: driver == "mysql"})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT `"+strings.Join(cols, "`, `")+"` FROM `"+in.Table+"` WHERE "+strings.Join(conds, " AND ")+" LIMIT 2", vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []map[string]*string
	for rows.Next() && len(found) < 2 {
		cells := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		r := map[string]*string{}
		for i, col := range cols {
			if cells[i].Valid {
				r[col] = &cells[i].String
			} else {
				r[col] = nil
			}
		}
		found = append(found, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("no row in %s matches %v", in.Table, in.Where)
	case 1:
		return found[0], nil
	}
	return nil, fmt.Errorf("more than one row in %s matches %v: add a column to where", in.Table, in.Where)
}

func (c *checker) run(ctx context.Context, in checkArgs) (string, error) {
	pw := password(c.s.url)
	out, err := c.compare(ctx, in)
	if err != nil {
		return "", errors.New(redact(err.Error(), pw))
	}
	return out, nil
}

// compare answers: a line, the counts, and a row only for each field that differs, each value cut at 40 characters. A row of the table is never shown whole.
func (c *checker) compare(ctx context.Context, in checkArgs) (string, error) {
	names := []string{in.Table}
	for k := range in.Where {
		names = append(names, k)
	}
	for _, col := range in.Map {
		names = append(names, col)
	}
	if len(in.Where) == 0 || len(in.Map) == 0 {
		return "", errors.New("where and map need at least one entry each")
	}
	for _, n := range names {
		if !validName.MatchString(n) {
			return "", fmt.Errorf("the name %q is not valid: letters, digits and _ only", n)
		}
	}
	req, err := c.find(in.Sent)
	if err != nil {
		return "", err
	}
	sent, err := object(req.Req, "", "sent body")
	if err != nil {
		return "", err
	}
	var read map[string]any
	if in.After != "" {
		rd, err := c.find(in.After)
		if err != nil {
			return "", err
		}
		if read, err = object(rd.Res, in.At, "answer that was read"); err != nil {
			return "", err
		}
	}
	fields := slices.Sorted(maps.Keys(in.Map))
	cols := make([]string, 0, len(fields))
	for _, f := range fields {
		if !slices.Contains(cols, in.Map[f]) {
			cols = append(cols, in.Map[f])
		}
	}
	stored, err := c.row(ctx, in, cols)
	if err != nil {
		return "", err
	}
	var equal, soft, differ int
	var emptied []string
	var b strings.Builder
	var picked []string
	for _, k := range slices.Sorted(maps.Keys(in.Where)) {
		picked = append(picked, k+"="+in.Where[k])
	}
	head := fmt.Sprintf("check %s -> %s[%s]", in.Sent, in.Table, strings.Join(picked, " "))
	if read != nil {
		head += " and " + in.After
	}
	rows := ""
	for _, f := range fields {
		sv, sok := sent[f]
		st := stored[in.Map[f]]
		cells := []seen{{}, {}}
		cells[0].text, cells[0].empty = value(sv, sok)
		cells[0].here = sok
		if st == nil {
			cells[1] = seen{`\N`, true, true}
		} else {
			cells[1] = seen{*st, *st == "", true}
		}
		if read != nil {
			rv, rok := read[f]
			t, e := value(rv, rok)
			cells = append(cells, seen{t, e, rok})
		}
		same, kinds := true, map[string]bool{}
		for _, k := range cells {
			if !k.here {
				same = false
			}
			if k.empty {
				kinds["empty"] = true
				continue
			}
			kinds["text"] = true
		}
		if same && kinds["empty"] && kinds["text"] {
			same = false
		}
		if same {
			ref := squash(cells[0].text)
			if cells[0].empty {
				ref = ""
			}
			for _, k := range cells[1:] {
				v := squash(k.text)
				if k.empty {
					v = ""
				}
				if v != ref {
					same = false
				}
			}
		}
		switch {
		case !same:
			differ++
			line := []string{f}
			for _, k := range cells {
				if !k.here {
					line = append(line, "(absent)")
					continue
				}
				line = append(line, show(strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(k.text)))
			}
			rows += strings.Join(line, "\t") + "\n"
		case !kinds["text"] && !sameText(cells[0].text, cells[1:]):
			soft++
			emptied = append(emptied, f)
		default:
			equal++
		}
	}
	fmt.Fprintf(&b, "%s\n%d fields: %d equal, %d empty~, %d different", head, len(fields), equal, soft, differ)
	if differ > 0 {
		b.WriteString("\nfield\tsent\tstored")
		if read != nil {
			b.WriteString("\tread")
		}
		b.WriteString("\n" + strings.TrimRight(rows, "\n"))
	}
	if soft > 0 {
		b.WriteString("\nempty~ (null, empty text or NULL, not the same): " + strings.Join(emptied, ", "))
	}
	return b.String(), nil
}

func (c *checker) tool(ctx context.Context, _ *mcp.CallToolRequest, in checkArgs) (*mcp.CallToolResult, any, error) {
	text, err := c.run(ctx, in)
	if err != nil {
		text = "Failed: " + strings.TrimRight(err.Error(), ". ") + ". Next: page(payloads) lists what was captured; query shows the columns."
	}
	return &mcp.CallToolResult{IsError: err != nil, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}
