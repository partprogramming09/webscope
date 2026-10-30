package code

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/partprogramming09/webscope/internal/fsx"
)

const columnSectionMax = 2000

var (
	insertCols  = regexp.MustCompile("(?is)insert\\s+(?:ignore\\s+)?into\\s+`?(\\w+)`?\\s*\\(([^)]*)\\)")
	updateSet   = regexp.MustCompile("(?is)\\bupdate\\s+`?(\\w+)`?\\s+set\\s+(.{1,400}?)(?:\\bwhere\\b|\"\\s*[;),]|$)")
	dupUpdate   = regexp.MustCompile(`(?is)^.{0,600}?on\s+duplicate\s+key\s+update\s+(.{1,300}?)(?:"\s*[;),]|;|$)`)
	assignedCol = regexp.MustCompile("(?:^|,)\\s*`?(\\w+)`?\\s*=")
	plainName   = regexp.MustCompile(`^\w+$`)
)

// columnsOf is the columns the SQL of a PHP file writes, by table, lower case: the list of an INSERT and the names assigned by an UPDATE or ON DUPLICATE KEY UPDATE.
func columnsOf(file string, all fsx.Lines) map[string][]string {
	var kept []string
	for _, l := range all[file] {
		if t := strings.TrimSpace(l); !strings.HasPrefix(t, "//") && !strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "*") && !strings.HasPrefix(t, "/*") {
			kept = append(kept, l)
		}
	}
	src := strings.Join(kept, "\n")
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

// unnamedColumns are the columns of cols that no PHP file of the backend names between quotes or after ->, case apart.
func (c *Tool) unnamedColumns(cols []string) []string {
	files, _, err := fsx.PHPFiles(c.Backend)
	if err != nil || len(cols) == 0 {
		return nil
	}
	quoted := make([]string, len(cols))
	for i, col := range cols {
		quoted[i] = regexp.QuoteMeta(col)
	}
	names := strings.Join(quoted, "|")
	re := regexp.MustCompile("(?i)(?:['\"\x60](" + names + ")['\"\x60]|->(" + names + ")\\b)")
	named := map[string]bool{}
	for _, h := range fsx.Grep(re, files, fsx.Lines{}) {
		for _, m := range re.FindAllStringSubmatch(h.Text, -1) {
			named[strings.ToLower(m[1]+m[2])] = true
		}
	}
	var none []string
	for _, col := range cols {
		if !named[col] {
			none = append(none, col)
		}
	}
	sort.Strings(none)
	return none
}

// columnGaps is the section of the legacy columns on shared tables that the backend never names, one line per table.
func (c *Tool) columnGaps(shared []string, written map[string][]string) string {
	head := "## columns the legacy writes and the backend never names (shared tables; a name found anywhere in the backend counts as named)"
	if c.Backend == "" {
		return head + "\nnot compared: WEBSCOPE_BACKEND is not set"
	}
	var all []string
	for _, t := range shared {
		all = append(all, written[t]...)
	}
	none := map[string]bool{}
	for _, col := range c.unnamedColumns(all) {
		none[col] = true
	}
	size, left := len(head), 0
	var lines []string
	for _, t := range shared {
		var miss []string
		for _, col := range written[t] {
			if none[col] {
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
		if size += len(l) + 1; size > columnSectionMax {
			left++
			continue
		}
		lines = append(lines, l)
	}
	if len(lines) == 0 {
		return head + "\n-"
	}
	if left > 0 {
		lines = append(lines, fmt.Sprintf("(+%d more tables)", left))
	}
	return head + "\n" + strings.Join(lines, "\n")
}
