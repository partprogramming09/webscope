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

var (
	phpMention  = regexp.MustCompile(`['"]([A-Za-z0-9_./-]+\.php)\b`)
	sqlTable    = regexp.MustCompile("(?i)\\b(delete\\s+from|insert\\s+(?:ignore\\s+)?into|replace\\s+into|update|from|join)\\s+`?([a-z_][a-z0-9_]*)")
	onDuplicate = regexp.MustCompile(`(?i)on\s+duplicate\s+key\s+update`)
	newTable    = regexp.MustCompile(`-> table (\w+)`)
	notTables   = map[string]bool{"select": true, "set": true, "dual": true, "where": true, "values": true, "as": true, "table": true, "information_schema": true, "lateral": true}
)

// legacySide is a PHP file of the legacy with the pages that call it and the tables its SQL names.
type legacySide struct {
	rel     string
	callers []string
	writes  []string
	reads   []string
	columns map[string][]string
}

// legacyDir is path inside WEBSCOPE_LEGACY, resolved: it must stay under the real root, so a link or a ../ cannot lead out of it.
func (c *Tool) legacyDir(path string) (real, dir string, err error) {
	if c.Legacy == "" {
		return "", "", errors.New("WEBSCOPE_LEGACY is not set, set it to the folder of the legacy PHP (the one that holds the module folders)")
	}
	p := filepath.ToSlash(filepath.Clean(path))
	if path == "" || p == "." || strings.HasPrefix(p, "/") || p == ".." || strings.HasPrefix(p, "../") {
		return "", "", errors.New("legacy needs a path inside WEBSCOPE_LEGACY, such as comercial/cotizadorX_new")
	}
	if real, err = filepath.EvalSymlinks(c.Legacy); err != nil {
		return "", "", err
	}
	if dir, err = filepath.EvalSymlinks(filepath.Join(real, p)); err != nil {
		return "", "", fmt.Errorf("no folder %q in WEBSCOPE_LEGACY", echo(path))
	}
	if rel, err := filepath.Rel(real, dir); err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", errors.New("the path leads out of WEBSCOPE_LEGACY")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", "", fmt.Errorf("%q is not a folder", echo(path))
	}
	return real, dir, nil
}

// tablesOf is the tables named by the SQL of a PHP file, in order of appearance: those it writes and those it only reads. Comment lines are skipped.
func tablesOf(file string, all fsx.Lines) (writes, reads []string) {
	w, r := map[string]bool{}, map[string]bool{}
	for _, h := range fsx.Grep(sqlTable, []string{file}, all) {
		if t := h.Text; strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*") {
			continue
		}
		for _, m := range sqlTable.FindAllStringSubmatch(onDuplicate.ReplaceAllString(h.Text, ""), -1) {
			name := strings.ToLower(m[2])
			if notTables[name] {
				continue
			}
			kw := strings.ToLower(strings.Fields(m[1])[0])
			if kw == "from" || kw == "join" {
				if !r[name] {
					r[name] = true
					reads = append(reads, name)
				}
			} else if !w[name] {
				w[name] = true
				writes = append(writes, name)
			}
		}
	}
	var onlyRead []string
	for _, n := range reads {
		if !w[n] {
			onlyRead = append(onlyRead, n)
		}
	}
	return writes, onlyRead
}

// legacy lists the PHP endpoints of a legacy folder with their callers and tables and, with module, compares those tables with the new module's: a text estimate.
func (c *Tool) legacy(args map[string]any) (string, error) {
	// shortcut: only the endpoint file is read, not its includes; follow them if tables are missing from a real answer
	path := input.Str(args, "path")
	real, dir, err := c.legacyDir(path)
	if err != nil {
		return "", err
	}
	files, _, err := fsx.WalkFiles(real, func(n string) bool {
		n = strings.ToLower(n)
		return strings.HasSuffix(n, ".php") || strings.HasSuffix(n, ".js") || strings.HasSuffix(n, ".html") || strings.HasSuffix(n, ".htm")
	})
	if err != nil {
		return "", err
	}
	all := fsx.Lines{}
	sides := map[string]*legacySide{}
	var order []string
	for _, f := range files {
		if rel, ok := strings.CutPrefix(f, dir+string(filepath.Separator)); ok && strings.HasSuffix(strings.ToLower(f), ".php") {
			rel = filepath.ToSlash(rel)
			s := &legacySide{rel: rel}
			s.writes, s.reads = tablesOf(f, all)
			s.columns = columnsOf(f, all)
			sides[rel] = s
			order = append(order, rel)
		}
	}
	sort.Strings(order)
	base := filepath.Base(dir)
	for _, h := range fsx.Grep(phpMention, files, all) {
		inside := strings.HasPrefix(h.File, dir+string(filepath.Separator))
		where := fmt.Sprintf("%s:%d", filepath.ToSlash(strings.TrimPrefix(h.File, real+string(filepath.Separator))), h.Line)
		for _, m := range phpMention.FindAllStringSubmatch(h.Text, -1) {
			var rel string
			if i := strings.Index(m[1], base+"/"); i >= 0 && (i == 0 || m[1][i-1] == '/') {
				rel = m[1][i+len(base)+1:]
			} else if inside {
				rel = strings.TrimPrefix(m[1], "./")
			}
			if s := sides[rel]; s != nil && !strings.HasPrefix(h.File, filepath.Join(dir, filepath.FromSlash(rel))) && !slicesContains(s.callers, where) {
				s.callers = append(s.callers, where)
			}
		}
	}
	var called, orphans []string
	var lines []string
	legacyTables := map[string]bool{}
	written := map[string][]string{}
	for _, rel := range order {
		s := sides[rel]
		for _, t := range append(append([]string{}, s.writes...), s.reads...) {
			legacyTables[t] = true
		}
		for t, cols := range s.columns {
			for _, col := range cols {
				if !slicesContains(written[t], col) {
					written[t] = append(written[t], col)
				}
			}
		}
		if len(s.callers) == 0 {
			orphans = append(orphans, rel)
			continue
		}
		called = append(called, rel)
		l := rel + " <- " + s.callers[0]
		if len(s.callers) > 1 {
			l += fmt.Sprintf(" (+%d)", len(s.callers)-1)
		}
		if len(s.writes) > 0 {
			l += "  W " + strings.Join(s.writes, ", ")
		}
		if len(s.reads) > 0 {
			l += "  R " + strings.Join(s.reads, ", ")
		}
		lines = append(lines, l)
	}
	out := []string{fmt.Sprintf("Legacy of «%s»: %d PHP files, %d with a caller. Text estimate: SQL built from variables or kept in an included file is missed. W = writes, R = only reads.", echo(path), len(order), len(called)), "## endpoints"}
	var tail []string
	if len(orphans) > 0 {
		more := ""
		if len(orphans) > 6 {
			more = fmt.Sprintf(" (+%d)", len(orphans)-6)
		}
		tail = append(tail, "no caller found: "+strings.Join(orphans[:min(6, len(orphans))], ", ")+more+" (pages loaded by address, includes or dead code)")
	}
	if module := input.Str(args, "module"); module != "" {
		tail = append(tail, c.againstNew(module, legacyTables, written))
	}
	room := input.MaxAnswer - len(strings.Join(out, "\n")) - len(strings.Join(tail, "\n")) - 80
	out = append(out, cutLines(strings.Join(lines, "\n"), room, "endpoints"))
	return strings.Join(append(out, tail...), "\n"), nil
}

// againstNew compares the tables of the legacy with the tables the models of the new module name, and the columns the legacy writes on the shared ones.
func (c *Tool) againstNew(module string, legacy map[string]bool, written map[string][]string) string {
	head := fmt.Sprintf("## against «%s» (tables of all its PHP files, not only the called ones)", echo(module))
	_, endpoints, found, err := c.flowParts(map[string]any{"module": module})
	if err != nil || !found {
		return head + "\nnot compared: no new module matches it. Next: module is a part of the path of a front module, such as crm/quotes"
	}
	nw := map[string]bool{}
	for _, m := range newTable.FindAllStringSubmatch(endpoints, -1) {
		nw[strings.ToLower(m[1])] = true
	}
	if len(nw) == 0 {
		return head + "\nnot compared: the new module names no table (" + firstNonEmpty(endpoints, "no endpoints") + ")"
	}
	var onlyL, onlyN, both []string
	for t := range legacy {
		if nw[t] {
			both = append(both, t)
		} else {
			onlyL = append(onlyL, t)
		}
	}
	for t := range nw {
		if !legacy[t] {
			onlyN = append(onlyN, t)
		}
	}
	list := func(l []string) string {
		sort.Strings(l)
		if len(l) > 60 {
			return strings.Join(l[:60], ", ") + fmt.Sprintf(" (+%d more)", len(l)-60)
		}
		return firstNonEmpty(strings.Join(l, ", "), "-")
	}
	left := "\nonly in legacy: " + list(onlyL)
	if elsewhere, none := c.namedInBackend(onlyL); c.Backend != "" {
		sort.Strings(none)
		var named []string
		for _, t := range onlyL {
			if n := elsewhere[t]; n > 0 {
				named = append(named, fmt.Sprintf("%s (%d %s)", t, n, map[bool]string{true: "file", false: "files"}[n == 1]))
			}
		}
		left = "\nonly in legacy and named elsewhere in the backend: " + firstNonEmpty(strings.Join(named, ", "), "-") +
			"\nonly in legacy and not named in the backend: " + list(none)
	}
	out := head + left + "\nonly in new: " + list(onlyN) + "\nboth: " + list(both)
	if len(both) > 0 {
		out += "\n" + c.columnGaps(both, written)
	}
	return out
}

// namedInBackend counts, for each table, the PHP files of WEBSCOPE_BACKEND that name it, and lists the tables no file names.
func (c *Tool) namedInBackend(tables []string) (count map[string]int, none []string) {
	count = map[string]int{}
	if c.Backend == "" || len(tables) == 0 {
		return count, nil
	}
	files, _, err := fsx.PHPFiles(c.Backend)
	if err != nil {
		return count, nil
	}
	quoted := make([]string, len(tables))
	for i, t := range tables {
		quoted[i] = regexp.QuoteMeta(t)
	}
	re := regexp.MustCompile(`\b(?:` + strings.Join(quoted, "|") + `)\b`)
	seen := map[string]bool{}
	for _, h := range fsx.Grep(re, files, fsx.Lines{}) {
		for _, name := range re.FindAllString(h.Text, -1) {
			if k := name + "\x00" + h.File; !seen[k] {
				seen[k] = true
				count[name]++
			}
		}
	}
	for _, t := range tables {
		if count[t] == 0 {
			none = append(none, t)
		}
	}
	return count, none
}
