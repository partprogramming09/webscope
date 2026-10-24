package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// codeSchema is written by hand, like checkSchema: the generated one costs twice the bytes of tools/list.
var codeSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"action": map[string]any{"type": "string"}, "args": map[string]any{"type": "object"},
	},
}

// coder reads the code of the project: the PHP under WEBSCOPE_BACKEND, and the columns of a table through the same read-only connection as query.
type coder struct {
	s       *source
	backend string
	front   string

	snapMu sync.Mutex
	snaps  map[string]*snapshot // in memory only: they are lost when the server restarts
}

type codeArgs struct {
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

const (
	maxPHP      = 20000   // files read at most
	maxPHPBytes = 2 << 20 // a bigger file is skipped
	maxConsumed = 25      // files listed by consumers
)

// skipDirs are never walked: dependencies, files the app writes, and the repository.
var skipDirs = map[string]bool{"vendor": true, "node_modules": true, "storage": true, ".git": true}

var writes = regexp.MustCompile(`(?i)\b(update|updateOrCreate|create|firstOrCreate|insert|save|fill|upsert)\w*\s*\(`)

// hit is one line that matched.
type hit struct {
	file string
	line int
	text string
}

// source of lines already read, by file.
type lines map[string][]string

// phpFiles are the .php files under root, read-only: symbolic links are never followed (a link could lead out of the root), .env* files, the
// folders in skipDirs and files over maxPHPBytes are left out. The names come back relative to the real root.
func phpFiles(root string) ([]string, string, error) {
	return walkFiles(root, func(n string) bool { return strings.HasSuffix(n, ".php") })
}

// walkFiles is the safe walk behind phpFiles for any kind of file: want says which names to keep.
func walkFiles(root string, want func(name string) bool) (files []string, real string, err error) {
	if real, err = filepath.EvalSymlinks(root); err != nil {
		return nil, "", err
	}
	err = filepath.WalkDir(real, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil || d.Type()&fs.ModeSymlink != 0:
			return nil
		case d.IsDir():
			if p != real && skipDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return nil
		case !d.Type().IsRegular(): // a fifo or a socket named X.php would block the read for ever
			return nil
		case !want(d.Name()) || strings.HasPrefix(strings.ToLower(d.Name()), ".env"):
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > maxPHPBytes {
			return nil
		}
		if files = append(files, p); len(files) >= maxPHP {
			return fs.SkipAll
		}
		return nil
	})
	return files, real, err
}

// grep returns the lines of files that match re, and the lines of every file it read.
func grep(re *regexp.Regexp, files []string, all lines) []hit {
	var out []hit
	for _, f := range files {
		ls, ok := all[f]
		if !ok {
			fh, err := os.Open(f)
			if err != nil {
				continue
			}
			sc := bufio.NewScanner(fh)
			sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
			for sc.Scan() {
				ls = append(ls, sc.Text())
			}
			fh.Close()
			all[f] = ls
		}
		for i, l := range ls {
			if re.MatchString(l) {
				out = append(out, hit{f, i + 1, strings.TrimSpace(l)})
			}
		}
	}
	return out
}

// classify says WRITES when an update, create, save, insert, fill or upsert call is within 3 lines of line n, READS otherwise: a text estimate.
func classify(ls []string, n int) string {
	if writes.MatchString(strings.Join(ls[max(0, n-4):min(len(ls), n+3)], " ")) {
		return "WRITES"
	}
	return "READS"
}

// tableFiles are the PHP files that name the table or the model that declares it ($table = 'name'), and the models.
func tableFiles(table string, files []string, all lines) ([]string, []string) {
	var models []string
	for _, h := range grep(regexp.MustCompile(`\$table\s*=\s*['"]`+regexp.QuoteMeta(table)+`['"]`), files, all) {
		if name := strings.TrimSuffix(filepath.Base(h.file), ".php"); name != "" {
			models = append(models, name)
		}
	}
	sort.Strings(models)
	names := []string{regexp.QuoteMeta(table)}
	for _, m := range models {
		names = append(names, regexp.QuoteMeta(m)) // a file name is not a pattern: "Foo (old.php" must not break the search
	}
	seen := map[string]bool{}
	var used []string
	for _, h := range grep(regexp.MustCompile(`\b(`+strings.Join(names, "|")+`)\b`), files, all) {
		if !seen[h.file] {
			seen[h.file] = true
			used = append(used, h.file)
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

func (c *coder) run(ctx context.Context, in codeArgs) (string, error) {
	out, err := c.do(ctx, in)
	if err != nil {
		return "", errors.New(redact(err.Error(), password(c.s.url)))
	}
	return out, nil
}

func str(args map[string]any, k string) string { s, _ := args[k].(string); return s }

func (c *coder) do(ctx context.Context, in codeArgs) (string, error) {
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
	table, col, only := str(args, "table"), str(args, "column"), str(args, "column_only")
	if only != "" {
		col = only
	}
	if col == "" || (only == "" && table == "") {
		return "", errors.New("consumers needs table and column, or column_only")
	}
	if only == "" && !validName.MatchString(table) {
		return "", fmt.Errorf("the table name %q is not valid: letters, digits and _ only", table)
	}
	files, real, err := phpFiles(c.backend)
	if err != nil {
		return "", err
	}
	all := lines{}
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
		fmt.Fprintf(&b, "NOTE: only the files that name the table or the model were searched; a Resource that reads $this->COLUMN may not name them: column_only searches the whole backend (more noise)\n")
	}
	if len([]rune(col)) < 4 {
		fmt.Fprintf(&b, "NOTE: '%s' is short; expect noise\n", col)
	}
	q := regexp.QuoteMeta(col)
	hits := grep(regexp.MustCompile(`['"`+"`"+`]`+q+`['"`+"`"+`]|->`+q+`\b`), files, all)
	per := map[string][]hit{}
	for _, h := range hits {
		per[h.file] = append(per[h.file], h)
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
	fmt.Fprintf(&b, "BACKEND  %d uses in %d files (an estimate: text search, READS or WRITES by the 3 lines around, not a flow analysis)\n", len(hits), len(per))
	for i, f := range names {
		if i == maxConsumed {
			fmt.Fprintf(&b, "  … and %d more files", len(names)-maxConsumed)
			break
		}
		marks := map[string]bool{}
		for _, h := range per[f] {
			marks[classify(all[f], h.line)] = true
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
		fmt.Fprintf(&b, "  %-13s %s:%d  %s%s\n", strings.Join(ms, "/"), rel, first.line, show70(first.text), extra)
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
	table := str(args, "table")
	if !validName.MatchString(table) {
		return "", fmt.Errorf("the table name %q is not valid: letters, digits and _ only", table)
	}
	db, driver, err := c.s.open(ctx, str(args, "tenant"))
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
	all := lines{}
	head := fmt.Sprintf("%s: %s rows", table, total)
	var models []string
	if c.backend == "" {
		head += " · WEBSCOPE_BACKEND is not set, so no readers or writers"
	} else {
		php, real, err := phpFiles(c.backend)
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
			var uses []hit
			if len(files) > 0 && len([]rune(k.name)) >= 3 {
				q := regexp.QuoteMeta(k.name)
				uses = grep(regexp.MustCompile(`['"`+"`"+`]`+q+`['"`+"`"+`]|->`+q+`\b`), files, all)
			}
			w, r := map[string]bool{}, map[string]bool{}
			for _, h := range uses {
				if classify(all[h.file], h.line) == "WRITES" {
					w[h.file] = true
				}
			}
			for _, h := range uses {
				if !w[h.file] {
					r[h.file] = true
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

func (c *coder) tool(ctx context.Context, _ *mcp.CallToolRequest, in codeArgs) (*mcp.CallToolResult, any, error) {
	text, err := c.run(ctx, in)
	if err != nil {
		text = "Failed: " + strings.TrimRight(err.Error(), ". ") + ". Next: code(help) lists the actions."
	}
	return &mcp.CallToolResult{IsError: err != nil, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}
