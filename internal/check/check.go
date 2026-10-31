package check

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
	"github.com/partprogramming09/webscope/internal/db"
	"github.com/partprogramming09/webscope/internal/input"
	"github.com/partprogramming09/webscope/internal/page"
)

// CaptureSource is where the requests the screen sent come from: the page tool.
type CaptureSource interface{ Captures() []page.Capture }

// Noter is a page source that wants to know what check found, so its verify can use it as evidence.
type Noter interface {
	NoteCheck(spec string, fields, differ, unsure int)
}

// Tool compares what the screen sent, what the database stored and what the screen read afterwards.
type Tool struct {
	Src  *db.Source
	Page CaptureSource
}

// Schema is written by hand: the one generated from checkArgs costs twice the bytes of tools/list. sent is a captured request
// ("POST /api/quotes"), where is {column: value} for one row, map is {sent field: column}, after is a captured read ("GET /api/quotes/1")
// and at the path of the object inside its answer ("quote", "data.0"). No "required" list: it costs bytes and run says what is missing.
var Schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"sent": map[string]any{"type": "string"}, "table": map[string]any{"type": "string"},
		"where": map[string]any{"type": "object"}, "map": map[string]any{"type": "object"},
		"after": map[string]any{"type": "string"}, "at": map[string]any{"type": "string"}, "tenant": map[string]any{"type": "string"},
	},
}

// textMap is {column: value} where a value written as a JSON number is kept as the digits it was written with.
type textMap map[string]string

// UnmarshalJSON reads a text or a number for each entry and refuses anything else, naming the entry.
func (m *textMap) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(textMap, len(raw))
	for k, v := range raw {
		var s string
		var n json.Number
		switch {
		case json.Unmarshal(v, &s) == nil && v[0] == '"':
			out[k] = s
		case json.Unmarshal(v, &n) == nil && v[0] != '"' && v[0] != 'n' && v[0] != 't' && v[0] != 'f':
			out[k] = n.String()
		default:
			return fmt.Errorf("where %q must be a text or a number", k)
		}
	}
	*m = out
	return nil
}

type checkArgs struct {
	Sent   string            `json:"sent"`
	Table  string            `json:"table"`
	Where  textMap           `json:"where"`
	Map    map[string]string `json:"map"`
	After  string            `json:"after,omitempty"`
	At     string            `json:"at,omitempty"`
	Tenant string            `json:"tenant,omitempty"`
}

// cutMark is what clip (here, in bytes) and page.js (in chars) leave at the end of a body they cut.
var cutMark = regexp.MustCompile(`…\(\+\d+ (bytes|chars)\)$`)

// find is the last captured request, at index from or later, that matches "METHOD /path": the path of the URL, or its end after a "/".
func (c *Tool) find(caps []page.Capture, spec string, from int) (page.Capture, int, error) {
	method, path, ok := strings.Cut(strings.TrimSpace(spec), " ")
	if !ok || path == "" {
		return page.Capture{}, 0, fmt.Errorf("%q is not METHOD /path", spec)
	}
	path = "/" + strings.TrimPrefix(path, "/")
	for i := len(caps) - 1; i >= from; i-- {
		k := caps[i]
		if u, err := url.Parse(k.U); err == nil && strings.EqualFold(k.M, method) && (u.Path == path || strings.HasSuffix(u.Path, path)) {
			return k, i, nil
		}
	}
	if from > 0 {
		return page.Capture{}, 0, fmt.Errorf("no captured request matches %q after the save (%d captured, page(payloads) lists them)", spec, len(caps))
	}
	return page.Capture{}, 0, fmt.Errorf("no captured request matches %q (%d captured, page(payloads) lists them)", spec, len(caps))
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

// row reads the one row of table that where picks, only the columns asked for, in a read-only transaction.
func (c *Tool) row(ctx context.Context, in checkArgs, cols []string) (map[string]*string, error) {
	conn, driver, err := c.Src.Open(ctx, in.Tenant)
	if err != nil {
		return nil, err
	}
	var conds []string
	var vals []any
	for _, k := range slices.Sorted(maps.Keys(in.Where)) {
		conds = append(conds, "`"+k+"` = ?")
		vals = append(vals, in.Where[k])
	}
	ctx, cancel := context.WithTimeout(ctx, db.QueryTimeout)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: driver == "mysql"})
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

func (c *Tool) run(ctx context.Context, in checkArgs) (string, error) {
	pw := db.Password(c.Src.URL)
	out, err := c.compare(ctx, in)
	if err != nil {
		return "", errors.New(db.Redact(err.Error(), pw))
	}
	return out, nil
}

// listNames answers a call that has a request and a table but no map: the names of the fields that were sent and of the columns of the table, and the
// names they share, never a value.
func (c *Tool) listNames(ctx context.Context, in checkArgs) (string, error) {
	if !db.ValidName.MatchString(in.Table) {
		return "", fmt.Errorf("the name %q is not valid: letters, digits and _ only", in.Table)
	}
	req, _, err := c.find(c.Page.Captures(), in.Sent, 0)
	if err != nil {
		return "", err
	}
	body, err := object(req.Req, "", "sent body")
	if err != nil {
		return "", err
	}
	conn, driver, err := c.Src.Open(ctx, in.Tenant)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, db.QueryTimeout)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: driver == "mysql"})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT * FROM `"+in.Table+"` LIMIT 0")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	fields := slices.Sorted(maps.Keys(body))
	var same []string
	for _, f := range fields {
		if slices.Contains(cols, f) {
			same = append(same, f)
		}
	}
	return fmt.Sprintf("no map given. Sent fields: %s. Columns of %s: %s. Same name on both sides: %s. Next: check {sent, table, where, map {sent field: column}}; where {column: value} picks the row.",
		strings.Join(fields, ", "), in.Table, strings.Join(cols, ", "), strings.Join(same, ", ")), nil
}

// compare answers: a line, the counts, and a row only for each field that differs, each value cut at 40 characters. A row of the table is never shown whole.
func (c *Tool) compare(ctx context.Context, in checkArgs) (string, error) {
	if len(in.Map) == 0 && in.Sent != "" && in.Table != "" {
		return c.listNames(ctx, in)
	}
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
		if !db.ValidName.MatchString(n) {
			return "", fmt.Errorf("the name %q is not valid: letters, digits and _ only", n)
		}
	}
	caps := c.Page.Captures() // one copy for both searches: the buffer may be trimmed between them
	req, at, err := c.find(caps, in.Sent, 0)
	if err != nil {
		return "", err
	}
	if req.S < 200 || req.S >= 300 {
		return "", fmt.Errorf("the request %s answered %d: nothing was saved, so there is nothing to compare", in.Sent, req.S)
	}
	sent, err := object(req.Req, "", "sent body")
	if err != nil {
		return "", err
	}
	var read map[string]any
	if in.After != "" {
		rd, _, err := c.find(caps, in.After, at)
		if err != nil {
			return "", err
		}
		if rd.S < 200 || rd.S >= 300 {
			return "", fmt.Errorf("the read %s answered %d: it shows nothing to compare", in.After, rd.S)
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
	var equal, soft, differ, spaces int
	var emptied, spaced, unreadNames []string
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
		shown, unread := cells, false
		if read != nil && cells[0].here && cells[1].here && !cells[2].here {
			// a read that does not name the field says nothing about it: sent and stored are compared alone
			cells, unread = cells[:2], true
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
		spacesOnly := false
		if same && kinds["text"] {
			for _, k := range cells[1:] {
				if !k.empty && k.text != cells[0].text {
					spacesOnly = true
				}
			}
		}
		switch {
		case spacesOnly:
			spaces++
			spaced = append(spaced, f)
		case !same:
			differ++
			line := []string{f}
			cells := shown
			for _, k := range cells {
				if !k.here {
					line = append(line, "(absent)")
					continue
				}
				line = append(line, input.Show(strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(k.text)))
			}
			rows += strings.Join(line, "\t") + "\n"
		case !kinds["text"] && !sameText(cells[0].text, cells[1:]):
			soft++
			emptied = append(emptied, f)
		default:
			if unread {
				unreadNames = append(unreadNames, f)
			} else {
				equal++
			}
		}
	}
	fmt.Fprintf(&b, "%s\n%d fields: %d equal, %d empty~, %d space~, %d different", head, len(fields), equal, soft, spaces, differ)
	if len(unreadNames) > 0 {
		fmt.Fprintf(&b, ", %d unread~", len(unreadNames))
	}
	if differ > 0 {
		b.WriteString("\nfield\tsent\tstored")
		if read != nil {
			b.WriteString("\tread")
		}
		b.WriteString("\n" + strings.TrimRight(rows, "\n"))
	}
	if spaces > 0 {
		b.WriteString("\nspace~ (the text is the same once the spaces are collapsed: the app trimmed or changed them): " + strings.Join(spaced, ", "))
	}
	if len(unreadNames) > 0 {
		b.WriteString("\nunread~ (the read does not name them; sent and stored agree): " + strings.Join(unreadNames, ", "))
	}
	if soft > 0 {
		b.WriteString("\nempty~ (null, empty text or NULL, not the same): " + strings.Join(emptied, ", "))
	}
	if n, ok := c.Page.(Noter); ok {
		n.NoteCheck(in.Sent, len(fields), differ, soft+spaces)
	}
	return b.String(), nil
}

func (c *Tool) Handle(ctx context.Context, _ *mcp.CallToolRequest, in checkArgs) (*mcp.CallToolResult, any, error) {
	text, err := c.run(ctx, in)
	if err != nil {
		text = "Failed: " + strings.TrimRight(err.Error(), ". ") + ". Next: page(payloads) lists what was captured; query shows the columns."
	}
	return &mcp.CallToolResult{IsError: err != nil, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}
