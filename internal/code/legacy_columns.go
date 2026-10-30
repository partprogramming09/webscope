package code

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/partprogramming09/webscope/internal/fsx"
)

const columnSectionMax = 2400

var (
	insertCols  = regexp.MustCompile("(?is)insert\\s+(?:ignore\\s+)?into\\s+`?(\\w+)`?\\s*\\(([^)]*)\\)")
	updateSet   = regexp.MustCompile("(?is)\\bupdate\\s+`?(\\w+)`?\\s+set\\s+(.{1,400}?)(?:\\bwhere\\b|\"\\s*[;),]|$)")
	dupUpdate   = regexp.MustCompile(`(?is)^.{0,600}?on\s+duplicate\s+key\s+update\s+(.{1,300}?)(?:"\s*[;),]|;|$)`)
	assignedCol = regexp.MustCompile("(?:^|,)\\s*`?(\\w+)`?\\s*=")
	plainName   = regexp.MustCompile(`^\w+$`)
	phpConcat   = regexp.MustCompile(`["']\s*\.\s*["']`)
	moduleDir   = regexp.MustCompile(`app/Modules/[^/\s]+/[^/\s]+/`)
)

// columnsOf is the columns the SQL of a PHP file writes, by table, lower case: the list of an INSERT and the names assigned by an UPDATE or ON DUPLICATE KEY UPDATE.
func columnsOf(file string, all fsx.Lines) map[string][]string {
	var kept []string
	for _, l := range all[file] {
		if t := strings.TrimSpace(l); !strings.HasPrefix(t, "//") && !strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "*") && !strings.HasPrefix(t, "/*") {
			kept = append(kept, l)
		}
	}
	src := phpConcat.ReplaceAllString(strings.Join(kept, "\n"), " ")
	out := map[string][]string{}
	add := func(table, col string) {
		table, col = strings.ToLower(table), strings.ToLower(col)
		if !notTables[table] && !slicesContains(out[table], col) {
			out[table] = append(out[table], col)
		}
	}
	assigned := func(table, segment string) {
		for _, m := range assignedCol.FindAllStringSubmatch(segment, -1) {
			add(table, m[1])
		}
	}
	for _, m := range insertCols.FindAllStringSubmatchIndex(src, -1) {
		table := src[m[2]:m[3]]
		for _, col := range strings.Split(src[m[4]:m[5]], ",") {
			if col = strings.Trim(strings.TrimSpace(col), "`"); plainName.MatchString(col) {
				add(table, col)
			}
		}
		if d := dupUpdate.FindStringSubmatch(src[m[1]:]); d != nil {
			assigned(table, d[1])
		}
	}
	for _, m := range updateSet.FindAllStringSubmatch(onDuplicate.ReplaceAllString(src, ";"), -1) {
		assigned(m[1], m[2])
	}
	return out
}

// moduleDirs are the folders app/Modules/Area/Module/ that the controllers, services and models of the new module live in, read from its endpoints text.
func moduleDirs(endpoints string) []string {
	var dirs []string
	for _, d := range moduleDir.FindAllString(endpoints, -1) {
		if !slicesContains(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// readOnlyFile is a file whose quoted `'name' =>` pairs describe data to send or to validate, not to store.
func readOnlyFile(path string) bool {
	return strings.Contains(path, "/Resources/") || strings.Contains(path, "/Requests/") ||
		strings.HasSuffix(path, "Resource.php") || strings.HasSuffix(path, "Request.php") || strings.HasSuffix(path, "Collection.php")
}

var (
	pairAfter   = regexp.MustCompile(`^\s*=>`)
	assignAfter = regexp.MustCompile(`^\s*(?:=[^=>]|\.=|\+=|-=)`)
	fillable    = regexp.MustCompile(`(?s)\$(?:fillable|guarded)\s*=\s*\[(.*?)\]`)
	quotedName  = regexp.MustCompile("['\"\x60](\\w+)['\"\x60]")
)

// namedColumns says, for each of cols, whether a PHP file of the backend names it between quotes or after ->, case apart (anywhere), whether it is assigned
// there (`'col' =>` outside Resources and Requests, `->col =`, a $fillable list: wrote), and whether such a file lies inside dirs (inModule).
func (c *Tool) namedColumns(cols, dirs []string) (anywhere, wrote, inModule map[string]bool) {
	anywhere, wrote, inModule = map[string]bool{}, map[string]bool{}, map[string]bool{}
	files, _, err := fsx.PHPFiles(c.Backend)
	if err != nil || len(cols) == 0 {
		return anywhere, wrote, inModule
	}
	quoted := make([]string, len(cols))
	for i, col := range cols {
		quoted[i] = regexp.QuoteMeta(col)
	}
	names := strings.Join(quoted, "|")
	re := regexp.MustCompile("(?i)(?:['\"\x60](" + names + ")['\"\x60]|->(" + names + ")\\b)")
	wanted := map[string]bool{}
	for _, col := range cols {
		wanted[col] = true
	}
	lines := fsx.Lines{}
	inside := func(path string) bool {
		for _, d := range dirs {
			if strings.Contains(path, "/"+d) {
				return true
			}
		}
		return false
	}
	seen := map[string]bool{}
	for _, h := range fsx.Grep(re, files, lines) {
		path := filepath.ToSlash(h.File)
		seen[h.File] = true
		for _, m := range re.FindAllStringSubmatchIndex(h.Text, -1) {
			name, after := "", ""
			if m[2] >= 0 {
				name, after = h.Text[m[2]:m[3]], h.Text[m[1]:]
			} else {
				name, after = h.Text[m[4]:m[5]], h.Text[m[1]:]
			}
			name = strings.ToLower(name)
			anywhere[name] = true
			isWrite := m[2] >= 0 && pairAfter.MatchString(after) && !readOnlyFile(path) || m[2] < 0 && assignAfter.MatchString(after)
			if isWrite {
				wrote[name] = true
				inModule[name] = inModule[name] || inside(path)
			}
		}
	}
	for f := range seen {
		for _, b := range fillable.FindAllStringSubmatch(strings.Join(lines[f], "\n"), -1) {
			for _, q := range quotedName.FindAllStringSubmatch(b[1], -1) {
				if name := strings.ToLower(q[1]); wanted[name] {
					wrote[name] = true
					inModule[name] = inModule[name] || inside(filepath.ToSlash(f))
				}
			}
		}
	}
	return anywhere, wrote, inModule
}

// columnLines is one line per table of the columns of written that pick selects, within room bytes.
func columnLines(shared []string, written map[string][]string, pick func(string) bool, room int) (lines []string, left int) {
	for _, t := range shared {
		var miss []string
		for _, col := range written[t] {
			if pick(col) {
				miss = append(miss, col)
			}
		}
		if len(miss) == 0 {
			continue
		}
		sort.Strings(miss)
		l := t + ": " + strings.Join(miss[:min(40, len(miss))], ", ")
		if len(miss) > 40 {
			l += fmt.Sprintf(" (+%d more)", len(miss)-40)
		}
		if room -= len(l) + 1; room < 0 {
			left++
			continue
		}
		lines = append(lines, l)
	}
	return lines, left
}

// columnGaps is the section of the legacy columns on shared tables that the backend does not name, in two groups: named nowhere, and named only outside the new module's folders.
func (c *Tool) columnGaps(shared []string, cols map[string][]string, endpoints string) string {
	head := "## columns the legacy writes on shared tables (named: a backend file has it between quotes or after ->; written: it is assigned there, in `'col' =>` outside Resources and Requests, in `->col =` or in a $fillable list)"
	if c.Backend == "" {
		return head + "\nnot compared: WEBSCOPE_BACKEND is not set"
	}
	var all []string
	tables := 0
	for _, t := range shared {
		if len(cols[t]) > 0 {
			tables++
		}
		all = append(all, cols[t]...)
	}
	dirs := moduleDirs(endpoints)
	shown := firstNonEmpty(strings.Join(dirs[:min(3, len(dirs))], ", "), "none found")
	if len(dirs) > 3 {
		shown += fmt.Sprintf(" (+%d)", len(dirs)-3)
	}
	head += fmt.Sprintf("\nchecked %d columns on %d tables; module folders: %s", len(all), tables, shown)
	anywhere, wrote, inModule := c.namedColumns(all, dirs)
	section := func(title string, pick func(string) bool) string {
		lines, left := columnLines(shared, cols, pick, columnSectionMax/3)
		if len(lines) == 0 {
			return title + " -"
		}
		if left > 0 {
			lines = append(lines, fmt.Sprintf("(+%d more tables)", left))
		}
		return title + "\n" + strings.Join(lines, "\n")
	}
	out := head + "\n" + section("named nowhere in the backend:", func(col string) bool { return !anywhere[col] })
	out += "\n" + section("named in the backend only where it is read, never assigned or filled (a Resource, a Request, a filter):", func(col string) bool { return anywhere[col] && !wrote[col] })
	if len(dirs) > 0 {
		out += "\n" + section("written in the backend, but not in the module's folders (who else writes them?):", func(col string) bool { return wrote[col] && !inModule[col] })
	}
	return out
}
