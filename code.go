package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/partprogramming09/webscope/internal/fsx"
	"github.com/partprogramming09/webscope/internal/input"
)

// coder reads the code of the project: the PHP under WEBSCOPE_BACKEND, and the columns of a table through the same read-only connection as query.
type coder struct {
	s       *source
	backend string
	front   string

	snapMu sync.Mutex
	snaps  map[string]*snapshot // in memory only: they are lost when the server restarts
}

const maxConsumed = 25 // files listed by consumers

var writes = regexp.MustCompile(`(?i)\b(update|updateOrCreate|create|firstOrCreate|insert|save|fill|upsert)\w*\s*\(`)

// classify says WRITES when line n assigns the column ($x->COL = v) or holds it as the key of an array ('COL' => v) within 3 lines of an update, create, save, insert, fill or upsert call, READS otherwise: a text estimate.
func classify(ls []string, n int, col string) string {
	q := regexp.QuoteMeta(col)
	if regexp.MustCompile(`->` + q + `\s*(\?\?|[-+.*/])?=(?:[^=>]|$)`).MatchString(ls[n-1]) {
		return "WRITES"
	}
	key := regexp.MustCompile(`['"` + "`" + `]` + q + `['"` + "`" + `]\s*=>`)
	if key.MatchString(ls[n-1]) && writes.MatchString(strings.Join(ls[max(0, n-4):min(len(ls), n+3)], " ")) {
		return "WRITES"
	}
	return "READS"
}

// tableFiles are the PHP files that name the table or the model that declares it ($table = 'name'), and the models.
func tableFiles(table string, files []string, all fsx.Lines) ([]string, []string) {
	var models []string
	var moduleDirs []string // the folder above Models/: its Resources/ read the columns of the model as $this->COLUMN, without naming the table or the model
	for _, h := range fsx.Grep(regexp.MustCompile(`\$table\s*=\s*['"]`+regexp.QuoteMeta(table)+`['"]`), files, all) {
		if name := strings.TrimSuffix(filepath.Base(h.File), ".php"); name != "" {
			models = append(models, name)
		}
		if filepath.Base(filepath.Dir(h.File)) == "Models" {
			moduleDirs = append(moduleDirs, filepath.Dir(filepath.Dir(h.File)))
		}
	}
	sort.Strings(models)
	names := []string{regexp.QuoteMeta(table)}
	for _, m := range models {
		names = append(names, regexp.QuoteMeta(m)) // a file name is not a pattern: "Foo (old.php" must not break the search
	}
	seen := map[string]bool{}
	var used []string
	for _, h := range fsx.Grep(regexp.MustCompile(`\b(`+strings.Join(names, "|")+`)\b`), files, all) {
		if !seen[h.File] {
			seen[h.File] = true
			used = append(used, h.File)
		}
	}
	for _, f := range files {
		for _, d := range moduleDirs {
			if !seen[f] && strings.HasPrefix(f, d+string(filepath.Separator)+"Resources"+string(filepath.Separator)) {
				seen[f] = true
				used = append(used, f)
			}
		}
	}
	return used, slicesCompact(models)
}

func slicesCompact(s []string) []string {
	var out []string
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func (c *coder) run(ctx context.Context, in input.Args) (string, error) {
	out, err := c.do(ctx, in)
	if err != nil {
		return "", errors.New(redact(err.Error(), password(c.s.url)))
	}
	return out, nil
}

func (c *coder) do(ctx context.Context, in input.Args) (string, error) {
	switch in.Action {
	case "consumers":
		return c.consumers(in.Args)
	case "fields":
		return c.fields(ctx, in.Args)
	case "snap":
		return c.snapSave(ctx, in.Args)
	case "snap_diff":
		return c.snapDiff(ctx, in.Args)
	case "map":
		return c.mapFront(in.Args)
	case "map_back":
		return c.mapBack(in.Args)
	}
	return "", fmt.Errorf("unknown action %q: fields {table, tenant?}, consumers {table, column} or {column_only}, map {module, walkthrough?}, map_back {query}, snap {name, sql, key?, tenant?} or snap_diff {name}", in.Action)
}

// consumers lists who reads and who writes a column in the PHP of WEBSCOPE_BACKEND.
func (c *coder) consumers(args map[string]any) (string, error) {
	if c.backend == "" {
		return "", errors.New("WEBSCOPE_BACKEND is not set")
	}
	table, col, only := input.Str(args, "table"), input.Str(args, "column"), input.Str(args, "column_only")
	if only != "" {
		col = only
	}
	if col == "" || (only == "" && table == "") {
		return "", errors.New("consumers needs table and column, or column_only")
	}
	if only == "" && !validName.MatchString(table) {
		return "", fmt.Errorf("the table name %q is not valid: letters, digits and _ only", table)
	}
	files, real, err := fsx.PHPFiles(c.backend)
	if err != nil {
		return "", err
	}
	all := fsx.Lines{}
	var b strings.Builder
	if only == "" {
		used, models := tableFiles(table, files, all)
		if len(used) == 0 {
			return "", fmt.Errorf("no backend file uses the table %s", table)
		}
		files = used
		m := strings.Join(models, ", ")
		if m == "" {
			m = "(no model; DB::table)"
		}
		fmt.Fprintf(&b, "model: %s · %d files use the table\n", m, len(used))
		fmt.Fprintf(&b, "NOTE: only the files that name the table or the model, and the Resources of their module, were searched; another file that reads $this->COLUMN may not name them: column_only searches the whole backend (more noise)\n")
	}
	if len([]rune(col)) < 4 {
		fmt.Fprintf(&b, "NOTE: '%s' is short; expect noise\n", col)
	}
	q := regexp.QuoteMeta(col)
	hits := fsx.Grep(regexp.MustCompile(`['"`+"`"+`]`+q+`['"`+"`"+`]|->`+q+`\b`), files, all)
	per := map[string][]fsx.Hit{}
	for _, h := range hits {
		per[h.File] = append(per[h.File], h)
	}
	names := make([]string, 0, len(per))
	for f := range per {
		names = append(names, f)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(per[names[i]]) != len(per[names[j]]) {
			return len(per[names[i]]) > len(per[names[j]])
		}
		return names[i] < names[j]
	})
	fmt.Fprintf(&b, "BACKEND  %d uses in %d files (an estimate: text search, READS or WRITES by the line of each use: an assignment, or a key of an array next to a write call; not a flow analysis)\n", len(hits), len(per))
	for i, f := range names {
		if i == maxConsumed {
			fmt.Fprintf(&b, "  … and %d more files", len(names)-maxConsumed)
			break
		}
		marks := map[string]bool{}
		for _, h := range per[f] {
			marks[classify(all[f], h.Line, col)] = true
		}
		var ms []string
		for m := range marks {
			ms = append(ms, m)
		}
		sort.Strings(ms)
		rel, _ := filepath.Rel(real, f)
		first, extra := per[f][0], ""
		if len(per[f]) > 1 {
			extra = fmt.Sprintf("  (+%d)", len(per[f])-1)
		}
		fmt.Fprintf(&b, "  %-13s %s:%d  %s%s\n", strings.Join(ms, "/"), rel, first.Line, show70(first.Text), extra)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func show70(s string) string {
	if r := []rune(s); len(r) > 70 {
		return string(r[:70])
	}
	return s
}

// fields is the sheet of a table: type, null, default, rows with a value of each column and, if WEBSCOPE_BACKEND is set, how many PHP files read
// and write each one. Counts only: no stored value is returned.
func (c *coder) fields(ctx context.Context, args map[string]any) (string, error) {
	table := input.Str(args, "table")
	if !validName.MatchString(table) {
		return "", fmt.Errorf("the table name %q is not valid: letters, digits and _ only", table)
	}
	db, driver, err := c.s.open(ctx, input.Str(args, "tenant"))
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: driver == "mysql"})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	type col struct{ name, kind, null, def string }
	var cols []col
	if driver == "mysql" {
		rows, err := tx.QueryContext(ctx, "SHOW COLUMNS FROM `"+table+"`")
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var f, t, n, k, e string
			var d sql.NullString
			if err := rows.Scan(&f, &t, &n, &k, &d, &e); err != nil {
				rows.Close()
				return "", err
			}
			cols = append(cols, col{f, t, n, d.String})
		}
		rows.Close()
	} else {
		rows, err := tx.QueryContext(ctx, "PRAGMA table_info(`"+table+"`)")
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var id, notnull, pk int
			var n, t string
			var d sql.NullString
			if err := rows.Scan(&id, &n, &t, &notnull, &d, &pk); err != nil {
				rows.Close()
				return "", err
			}
			null := "YES"
			if notnull == 1 {
				null = "NO"
			}
			cols = append(cols, col{n, t, null, d.String})
		}
		rows.Close()
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("the table %s has no columns or does not exist", table)
	}
	sel := []string{"COUNT(*)"}
	for _, k := range cols {
		if validName.MatchString(k.name) {
			sel = append(sel, "SUM(CASE WHEN `"+k.name+"` IS NOT NULL AND CAST(`"+k.name+"` AS CHAR) <> '' THEN 1 ELSE 0 END)")
		} else {
			sel = append(sel, "NULL")
		}
	}
	vals := make([]sql.NullString, len(sel))
	ptrs := make([]any, len(sel))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := tx.QueryRowContext(ctx, "SELECT "+strings.Join(sel, ", ")+" FROM `"+table+"`").Scan(ptrs...); err != nil {
		return "", err
	}
	total := vals[0].String
	var b strings.Builder
	var files []string
	all := fsx.Lines{}
	head := fmt.Sprintf("%s: %s rows", table, total)
	var models []string
	if c.backend == "" {
		head += " · WEBSCOPE_BACKEND is not set, so no readers or writers"
	} else {
		php, real, err := fsx.PHPFiles(c.backend)
		if err != nil {
			return "", err
		}
		_ = real
		files, models = tableFiles(table, php, all)
		m := strings.Join(models, ", ")
		if m == "" {
			m = "—"
		}
		head += fmt.Sprintf(" · model %s · %d backend files use it", m, len(files))
	}
	b.WriteString(head + "\ncolumn\ttype\tnull\tdefault\twith value\treaders / writers")
	for i, k := range cols {
		with := "?"
		if vals[i+1].Valid {
			with = vals[i+1].String
		} else if validName.MatchString(k.name) {
			with = "0"
		}
		users, note := "-", ""
		if c.backend != "" {
			var uses []fsx.Hit
			if len(files) > 0 && len([]rune(k.name)) >= 3 {
				q := regexp.QuoteMeta(k.name)
				uses = fsx.Grep(regexp.MustCompile(`['"`+"`"+`]`+q+`['"`+"`"+`]|->`+q+`\b`), files, all)
			}
			w, r := map[string]bool{}, map[string]bool{}
			for _, h := range uses {
				if classify(all[h.File], h.Line, k.name) == "WRITES" {
					w[h.File] = true
				}
			}
			for _, h := range uses {
				if !w[h.File] {
					r[h.File] = true
				}
			}
			users = strconv.Itoa(len(r)) + " / " + strconv.Itoa(len(w))
			if len(uses) == 0 && len([]rune(k.name)) >= 3 {
				note = "nobody names it"
			}
		}
		if with == "0" && total != "0" {
			note = strings.TrimLeft(note+"; no value in any row", "; ")
		}
		line := strings.Join([]string{k.name, k.kind, k.null, k.def, with + "/" + total, users}, "\t")
		if note != "" {
			line += "\t← " + note
		}
		b.WriteString("\n" + line)
	}
	out := b.String()
	if len(out) > maxBytes {
		cut := strings.LastIndex(out[:maxBytes], "\n")
		out = out[:cut] + fmt.Sprintf("\n(%d of %d columns shown, cut at %d bytes)", strings.Count(out[:cut], "\n")-1, len(cols), maxBytes)
	}
	return out, nil
}

func (c *coder) tool(ctx context.Context, _ *mcp.CallToolRequest, in input.Args) (*mcp.CallToolResult, any, error) {
	text, err := c.run(ctx, in)
	if err != nil {
		text = "Failed: " + strings.TrimRight(err.Error(), ". ") + ". Next: code(help) lists the actions."
	}
	return &mcp.CallToolResult{IsError: err != nil, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}
