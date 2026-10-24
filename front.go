package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	routePath  = regexp.MustCompile(`path:\s*['"]([^'"]+)['"]`)
	routeName  = regexp.MustCompile(`name:\s*['"]([^'"]+)['"]`)
	routeView  = regexp.MustCompile(`import\(['"]([^'"]+\.vue)['"]\)`)
	httpCall   = regexp.MustCompile("\\b(?:axios|http|api)\\.(get|post|put|patch|delete)(?:<[^>]*>)?\\(\\s*(?:`([^`]+)`|'([^']+)'|\"([^\"]+)\")")
	tagRe      = regexp.MustCompile(`<([A-Za-z][\w-]*)((?:\s+[^\s=>/"']+(?:=(?:"[^"]*"|'[^']*'))?)*)\s*/?>([^<]*)`)
	attrRe     = regexp.MustCompile(`([^\s=>/"']+)(?:=(?:"([^"]*)"|'([^']*)'))?`)
	importRe   = regexp.MustCompile(`import\s+(\w+)\s+from\s+['"]([^'"]+\.vue)['"]`)
	composable = regexp.MustCompile(`from\s+['"]([^'"]*/composables/\w+)['"]`)
	statusRe   = regexp.MustCompile(`(?m)^\s*([A-Z][A-Z_]+):`)
	i18nRe     = regexp.MustCompile(`\$?\bt\(\s*['"]([^'"]+)`)
)

// frontRoute is a route of a Routes.ts file; at is "file:line" relative to the modules folder.
type frontRoute struct{ path, name, view, at string }

// frontModule is what one module (its first two folders) holds.
type frontModule struct {
	routes              []frontRoute
	calls               []string // "GET /api/x  file:line"
	composables, stores []string
}

// frontBase is the folder of the modules: WEBSCOPE_FRONT/src/modules when it exists, else WEBSCOPE_FRONT; and the real root.
func (c *coder) frontBase() (base, root string, err error) {
	if c.front == "" {
		return "", "", errors.New("WEBSCOPE_FRONT is not set")
	}
	if root, err = filepath.EvalSymlinks(c.front); err != nil {
		return "", "", err
	}
	base = root
	if d, err := filepath.EvalSymlinks(filepath.Join(root, "src", "modules")); err == nil && strings.HasPrefix(d, root+string(filepath.Separator)) {
		base = d
	}
	return base, root, nil
}

func lineAt(text string, i int) int { return strings.Count(text[:i], "\n") + 1 }

// indexModules reads the .ts and .vue files under base into modules, in the order of their names.
func indexModules(base string) (map[string]*frontModule, []string, error) {
	files, _, err := walkFiles(base, func(n string) bool { return strings.HasSuffix(n, ".ts") || strings.HasSuffix(n, ".vue") })
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(files)
	mods := map[string]*frontModule{}
	var names []string
	for _, f := range files {
		rel, _ := filepath.Rel(base, f)
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		name := parts[0]
		if len(parts) > 2 {
			name = parts[0] + "/" + parts[1]
		}
		m := mods[name]
		if m == nil {
			m = &frontModule{}
			mods[name], names = m, append(names, name)
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := string(raw)
		if strings.HasSuffix(f, "Routes.ts") {
			blocks := strings.Split(text, "{")
			before := strings.Count(blocks[0], "\n")
			for _, b := range blocks[1:] {
				if p := routePath.FindStringSubmatchIndex(b); p != nil {
					r := frontRoute{path: b[p[2]:p[3]], name: "-", view: "-", at: fmt.Sprintf("%s:%d", rel, before+1+strings.Count(b[:p[0]], "\n"))}
					if n := routeName.FindStringSubmatch(b); n != nil {
						r.name = n[1]
					}
					if v := routeView.FindStringSubmatch(b); v != nil {
						r.view = v[1]
					}
					m.routes = append(m.routes, r)
				}
				before += strings.Count(b, "\n")
			}
		}
		if slicesContains(parts, "composables") {
			m.composables = append(m.composables, rel)
		}
		if slicesContains(parts, "stores") {
			m.stores = append(m.stores, rel)
		}
		// The whole text, not line by line: a call with a generic often has its URL on the next line. The line is where the call starts.
		for _, h := range httpCall.FindAllStringSubmatchIndex(text, -1) {
			var url string
			for g := 4; g <= 8; g += 2 {
				if h[g] >= 0 {
					url = text[h[g]:h[g+1]]
				}
			}
			m.calls = append(m.calls, fmt.Sprintf("%s %s  %s:%d", strings.ToUpper(text[h[2]:h[3]]), url, rel, lineAt(text, h[0])))
		}
	}
	return mods, names, nil
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// capOut cuts an answer at maxBytes, on a line, and says so.
func capOut(s string) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := strings.LastIndex(s[:maxBytes], "\n")
	if cut < 0 { // no line break in the first maxBytes: cut at a character
		for cut = maxBytes; cut > 0 && !utf8.RuneStart(s[cut]); cut-- {
		}
	}
	return s[:cut] + fmt.Sprintf("\n(cut at %d bytes: ask for a narrower module)", maxBytes)
}

// mapFront is map: the routes, calls, composables and stores of the modules whose name contains module; with walkthrough, what can be clicked.
func (c *coder) mapFront(args map[string]any) (string, error) {
	filter := str(args, "module")
	if filter == "" {
		return "", errors.New("map needs a module, a part of the path such as configs/templates")
	}
	base, _, err := c.frontBase()
	if err != nil {
		return "", err
	}
	mods, names, err := indexModules(base)
	if err != nil {
		return "", err
	}
	walk, _ := args["walkthrough"].(bool)
	out := []string{fmt.Sprintf("Frontend of «%s»: routes (screens), api (calls to the backend), composables and stores. Next: map_back.", echo(filter))}
	for _, n := range names {
		if !strings.Contains(n, filter) {
			continue
		}
		m := mods[n]
		out = append(out, "## "+n)
		for _, r := range m.routes {
			out = append(out, fmt.Sprintf("route %s (%s) -> %s  %s", r.path, r.name, r.view, r.at))
		}
		if walk {
			out = append(out, fmt.Sprintf("api total %d", len(m.calls)))
			out = append(out, walkthrough(base, mods, n)...)
			continue
		}
		for _, call := range m.calls {
			out = append(out, "api "+call)
		}
		for _, x := range m.composables {
			out = append(out, "composable "+x)
		}
		for _, x := range m.stores {
			out = append(out, "store "+x)
		}
		if len(m.stores) == 0 {
			out = append(out, "stores: none")
		}
	}
	if len(out) == 1 {
		return fmt.Sprintf("no module contains «%s»", echo(filter)), nil
	}
	return capOut(strings.Join(out, "\n")), nil
}

// importTarget is an import (`@/…` or relative to file) relative to base, or "" if it falls outside.
func importTarget(base, file, spec string) string {
	var t string
	switch {
	case strings.HasPrefix(spec, "@/"):
		t = filepath.Join(filepath.Dir(base), spec[2:])
	case strings.HasPrefix(spec, "."):
		t = filepath.Join(filepath.Dir(file), spec)
	default:
		return ""
	}
	rel, err := filepath.Rel(base, t)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}

// composablesSummary is "useX: GET×3 POST" for each composable that text imports and that makes HTTP calls.
func composablesSummary(base, file, text string, byAPI map[string][]string) string {
	var parts []string
	seen := map[string]bool{}
	for _, m := range composable.FindAllStringSubmatch(text, -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		rel := importTarget(base, file, m[1])
		methods := byAPI[rel+".ts"]
		if rel == "" || len(methods) == 0 {
			continue
		}
		var order []string
		count := map[string]int{}
		for _, x := range methods {
			if count[x] == 0 {
				order = append(order, x)
			}
			count[x]++
		}
		var ms []string
		for _, x := range order {
			if count[x] > 1 {
				x += "×" + strconv.Itoa(count[x])
			}
			ms = append(ms, x)
		}
		parts = append(parts, filepath.Base(m[1])+": "+strings.Join(ms, " "))
	}
	return strings.Join(parts, "; ")
}

type tabInfo struct {
	from, line int
	value      string
	comps      []string
}

// walkthrough lists, for the files of a module, the actions with their condition, the permissions, links, tabs, fields and statuses.
// shortcut: it does not inherit the v-if of a parent <template>, and a tab shows its components without their API; improve if the live walkthrough misses them.
func walkthrough(base string, mods map[string]*frontModule, module string) []string {
	byAPI := map[string][]string{}
	for _, m := range mods {
		for _, call := range m.calls {
			i := strings.LastIndex(call, "  ")
			file := call[i+2 : strings.LastIndex(call, ":")]
			byAPI[file] = append(byAPI[file], strings.Fields(call)[0])
		}
	}
	files, _, err := walkFiles(filepath.Join(base, module), func(n string) bool { return strings.HasSuffix(n, ".ts") || strings.HasSuffix(n, ".vue") })
	if err != nil {
		return nil
	}
	sort.Strings(files)
	var out []string
	for _, f := range files {
		rel, _ := filepath.Rel(base, f)
		rel = filepath.ToSlash(rel)
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := string(raw)
		if strings.HasSuffix(f, ".ts") {
			if strings.Contains(filepath.Base(f), "Status") {
				var st []string
				seen := map[string]bool{}
				for _, m := range statusRe.FindAllStringSubmatch(text, -1) {
					if !seen[m[1]] {
						seen[m[1]] = true
						st = append(st, m[1])
					}
				}
				if len(st) > 0 {
					out = append(out, "statuses "+strings.Join(st, " ")+"  "+rel)
				}
			}
			continue
		}
		start, end := strings.Index(text, "<template"), strings.LastIndex(text, "</template>")
		if start < 0 || end < start {
			continue
		}
		template := text[start:end]
		imports := map[string]string{}
		for _, m := range importRe.FindAllStringSubmatch(text, -1) {
			imports[m[1]] = m[2]
		}
		var closings []int
		for _, m := range regexp.MustCompile(`</v-window>`).FindAllStringIndex(template, -1) {
			closings = append(closings, m[0])
		}
		var lines []any
		var fields []string
		firstField := 0
		var tab *tabInfo
		for _, t := range tagRe.FindAllStringSubmatchIndex(template, -1) {
			name, attrs, after := template[t[2]:t[3]], template[t[4]:t[5]], template[t[6]:t[7]]
			n := lineAt(text, start+t[0])
			type attr struct{ k, v string }
			var al []attr
			a := map[string]string{}
			for _, x := range attrRe.FindAllStringSubmatchIndex(attrs, -1) {
				k := attrs[x[2]:x[3]]
				v := ""
				if x[4] >= 0 {
					v = attrs[x[4]:x[5]]
				} else if x[6] >= 0 {
					v = attrs[x[6]:x[7]]
				}
				al = append(al, attr{k, v})
				a[k] = v
			}
			click := ""
			for _, x := range al {
				if k := strings.SplitN(x.k, ".", 2)[0]; k == "@click" || k == "v-on:click" {
					click = x.v
					break
				}
			}
			if tab != nil {
				for _, cl := range closings {
					if tab.from < cl && cl < t[0] {
						tab = nil
						break
					}
				}
			}
			if name == "v-window-item" {
				v := a["value"]
				if v == "" {
					v = a[":value"]
				}
				tab = &tabInfo{from: t[0], line: n, value: v}
				lines = append(lines, tab)
			} else if tab != nil && name[0] >= 'A' && name[0] <= 'Z' {
				tab.comps = append(tab.comps, name)
			}
			if click != "" {
				label := firstNonEmpty(a[":title"], a["title"], a["icon"], strings.TrimSpace(after))
				l := fmt.Sprintf("action %d %s", n, click)
				if v, ok := a["v-if"]; ok {
					l += " if " + v
				}
				if label != "" {
					l += " «" + label + "»"
				}
				lines = append(lines, l)
			}
			attrStart := start + t[4]
			pos := 0
			for _, x := range al {
				at := strings.Index(attrs[pos:], x.k) + pos
				pos = at + len(x.k)
				if strings.Contains(x.v, "can(") && !(click != "" && x.k == "v-if") {
					lines = append(lines, fmt.Sprintf("permission %d %s=%s", lineAt(text, attrStart+at), x.k, x.v))
				}
			}
			if name == "AppLink" {
				lines = append(lines, fmt.Sprintf("link %d %s", n, firstNonEmpty(a[":to"], a["to"])))
			}
			if _, ok := a[":label"]; ok || hasKey(a, "label") {
				value := firstNonEmpty(a[":label"], a["label"])
				key := value
				if m := i18nRe.FindStringSubmatch(value); m != nil {
					key = m[1]
				}
				key = key[strings.LastIndex(key, ".")+1:]
				if hasKey(a, "required") || strings.Contains(a[":rules"], "required") {
					key += "*"
				}
				if len(fields) == 0 {
					firstField = n
				}
				fields = append(fields, key)
			}
		}
		if len(fields) > 0 {
			lines = append(lines, fmt.Sprintf("fields %d %s", firstField, strings.Join(fields, ", ")))
		}
		api := composablesSummary(base, f, text, byAPI)
		if len(lines) > 0 || api != "" {
			h := "### " + rel
			if api != "" {
				h += " [" + api + "]"
			}
			out = append(out, h)
		}
		for _, l := range lines {
			switch v := l.(type) {
			case *tabInfo:
				var comps []string
				seen := map[string]bool{}
				for _, cn := range v.comps {
					if seen[cn] {
						continue
					}
					seen[cn] = true
					if t := importTarget(base, f, imports[cn]); imports[cn] != "" && t != "" {
						cn += " " + t
					}
					comps = append(comps, cn)
				}
				out = append(out, fmt.Sprintf("tab %d %s -> %s", v.line, v.value, strings.Join(comps, ", ")))
			case string:
				out = append(out, v)
			}
		}
	}
	return out
}

func hasKey(m map[string]string, k string) bool { _, ok := m[k]; return ok }

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

// echo is what the user typed, for an answer: at most 80 characters.
func echo(s string) string {
	if r := []rune(s); len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return s
}
