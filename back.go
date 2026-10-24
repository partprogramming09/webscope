package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	routeCall   = regexp.MustCompile(`(?s)Route::(get|post|put|patch|delete|apiResource)\(\s*'([^']*)'\s*,\s*\[?\s*([\w\\]+)::class(?:\s*,\s*'(\w+)')?[^;]*;`)
	routePrefix = regexp.MustCompile(`(?s)Route::prefix\('([^']+)'\)[^;{]*?->group\(\s*function\s*\(\)\s*\{`)
	useRe       = regexp.MustCompile(`(?m)^use ([\w\\]+);`)
	limitRe     = regexp.MustCompile(`->(only|except)\(\[([^\]]*)\]\)`)
	paramRe     = regexp.MustCompile(`\$\{[^}]*\}|\{[^}]*\}`)
	camelRe     = regexp.MustCompile(`([a-z])([A-Z])`)
	wordRe      = regexp.MustCompile(`[a-z]+`)
	tableRe     = regexp.MustCompile(`\$table\s*=\s*['"]([^'"]+)['"]`)
	resource    = []struct{ name, verb, suffix string }{{"index", "GET", ""}, {"store", "POST", ""}, {"show", "GET", "/{}"}, {"update", "PUT", "/{}"}, {"destroy", "DELETE", "/{}"}}
)

// backRoute is a route of the backend; action is "Class@method".
type backRoute struct{ method, uri, action string }

// normalize is an address without `api/`, without `?`, without the outer `/` and with each parameter as `{}`.
func normalize(uri string) string {
	uri, _, _ = strings.Cut(uri, "?")
	return strings.TrimPrefix(strings.Trim(paramRe.ReplaceAllString(uri, "{}"), "/"), "api/")
}

// callKey is the address of a front call as a route is written: parameters as {}, and a URL that ends in a slash (axios.get('quotes/' + id): an id is
// added after it) means `quotes/{}`, not `quotes`.
func callKey(url string) string {
	n := normalize(url)
	if base, _, _ := strings.Cut(url, "?"); n != "" && strings.HasSuffix(base, "/") {
		n += "/{}"
	}
	return n
}

// stem is the singular of an English word, enough to match `services` with `service`.
func stem(w string) string {
	switch {
	case strings.HasSuffix(w, "ies") && len(w) > 4:
		return w[:len(w)-3] + "y"
	case regexp.MustCompile(`(ss|us|is)$`).MatchString(w):
		return w
	case regexp.MustCompile(`(ses|xes|zes|ches|shes)$`).MatchString(w):
		return w[:len(w)-2]
	case strings.HasSuffix(w, "s") && len(w) > 3:
		return w[:len(w)-1]
	}
	return w
}

// tokens are the singular lowercase words of a name in any style (`Configs/ServiceTypes`, `service-types`).
func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(camelRe.ReplaceAllString(s, "$1 $2")), -1) {
		out[stem(w)] = true
	}
	return out
}

// matches says whether every word of query is a word of name.
func matches(query, name string) bool {
	q, n := tokens(query), tokens(name)
	for w := range q {
		if !n[w] {
			return false
		}
	}
	return len(q) > 0
}

// backendModule is `Area/Module` of a controller class `App\Modules\Area\Module\Controllers\X@m`, or "".
func backendModule(action string) string {
	parts := strings.Split(strings.SplitN(action, "@", 2)[0], `\`)
	for i, p := range parts {
		if p == "Modules" && i+2 < len(parts) {
			return parts[i+1] + "/" + parts[i+2]
		}
	}
	return ""
}

// fileRoutes deduces the routes from the files app/Modules/**/Routes/*.php; nothing is executed.
// shortcut: apiResource parameters as `{}`, groups only with Route::prefix, `api/` assumed; improve if there are routes outside that pattern.
func fileRoutes(files []string, all lines) []backRoute {
	var routes []backRoute
	sort.Strings(files)
	for _, f := range files {
		if filepath.Base(filepath.Dir(f)) != "Routes" || !strings.Contains(filepath.ToSlash(f), "/app/Modules/") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := string(raw)
		uses := map[string]string{}
		for _, m := range useRe.FindAllStringSubmatch(text, -1) {
			uses[m[1][strings.LastIndex(m[1], `\`)+1:]] = m[1]
		}
		type span struct {
			a, b   int
			prefix string
		}
		var spans []span
		for _, m := range routePrefix.FindAllStringSubmatchIndex(text, -1) {
			level, j := 1, m[1]
			for j < len(text) && level > 0 {
				switch text[j] {
				case '{':
					level++
				case '}':
					level--
				}
				j++
			}
			spans = append(spans, span{m[0], j, strings.Trim(text[m[2]:m[3]], "/")})
		}
		for _, m := range routeCall.FindAllStringSubmatchIndex(text, -1) {
			kind, uri, cls, method := text[m[2]:m[3]], strings.Trim(text[m[4]:m[5]], "/"), text[m[6]:m[7]], ""
			if m[8] >= 0 {
				method = text[m[8]:m[9]]
			}
			if full, ok := uses[cls[strings.LastIndex(cls, `\`)+1:]]; ok {
				cls = full
			}
			var parts []string
			for _, s := range spans {
				if s.a < m[0] && m[0] < s.b {
					parts = append(parts, s.prefix)
				}
			}
			if uri != "" {
				parts = append(parts, uri)
			}
			full := strings.Join(parts, "/")
			if kind == "apiResource" {
				limit := limitRe.FindStringSubmatch(text[m[0]:m[1]])
				names := map[string]bool{}
				if limit != nil {
					for _, n := range regexp.MustCompile(`'(\w+)'`).FindAllStringSubmatch(limit[2], -1) {
						names[n[1]] = true
					}
				}
				for _, r := range resource {
					if limit == nil || names[r.name] == (limit[1] == "only") {
						routes = append(routes, backRoute{r.verb, normalize(full + r.suffix), cls + "@" + r.name})
					}
				}
				continue
			}
			if method == "" {
				method = "__invoke"
			}
			routes = append(routes, backRoute{strings.ToUpper(kind), normalize(full), cls + "@" + method})
		}
	}
	return routes
}

// fileOf is the PHP file of the class `App\…\Name`, or "" if it does not exist, is not a plain file, is too big, is a .env, is under a folder
// the walk skips, or once its symbolic links are resolved is not under root (root is already the real path of the backend).
func fileOf(root, cls string) string {
	rel := strings.ReplaceAll(strings.Replace(cls, `App\`, "app/", 1), `\`, "/") + ".php"
	if skipped(rel) {
		return ""
	}
	real, err := filepath.EvalSymlinks(filepath.Join(root, rel))
	if err != nil || !strings.HasPrefix(real, root+string(filepath.Separator)) {
		return ""
	}
	if inside, err := filepath.Rel(root, real); err != nil || skipped(filepath.ToSlash(inside)) { // the links are resolved: the same rules apply to where they lead
		return ""
	}
	if fi, err := os.Stat(real); err != nil || !fi.Mode().IsRegular() || fi.Size() > maxPHPBytes {
		return ""
	}
	return real
}

// skipped says whether a path relative to the root goes through a folder the walk skips, an empty or ".." part, or is a .env, in any letter case.
func skipped(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if l := strings.ToLower(part); part == "" || part == ".." || skipDirs[l] || strings.HasPrefix(l, ".env") {
			return true
		}
	}
	return false
}

func importsOf(file, folder string) []string {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range regexp.MustCompile(`(?m)^use ([\w\\]*\\`+folder+`\\[\w\\]+)(?: as \w+)?;`).FindAllStringSubmatch(string(raw), -1) {
		out = append(out, m[1])
	}
	return out
}

// mapBack is map_back: from the screens of the front to the endpoint, controller (with line), service, model and table.
func (c *coder) mapBack(args map[string]any) (string, error) {
	query := str(args, "query")
	if query == "" {
		return "", errors.New("map_back needs a query: a front module or a part of an API address")
	}
	if c.backend == "" {
		return "", errors.New("WEBSCOPE_BACKEND is not set")
	}
	base, froot, err := c.frontBase()
	if err != nil {
		return "", err
	}
	files, root, err := phpFiles(c.backend)
	if err != nil {
		return "", err
	}
	routes := fileRoutes(files, lines{})
	mods, names, err := indexModules(base)
	if err != nil {
		return "", err
	}
	frontRel, _ := filepath.Rel(froot, base)
	calls := map[string][]string{} // "METHOD uri" -> where
	for _, n := range names {
		for _, call := range mods[n].calls {
			head, where, _ := strings.Cut(call, "  ")
			method, url, _ := strings.Cut(head, " ")
			k := method + " " + callKey(url)
			calls[k] = append(calls[k], filepath.ToSlash(filepath.Join(frontRel, where)))
		}
	}
	out := []string{fmt.Sprintf("Backend of «%s»: from the frontend route to the endpoint (API address), controller, service and table. Next: read(controller, method).", echo(query)),
		"routes read from: files Routes/*.php (artisan is not run)"}
	var chosen []backRoute
	in := func(r backRoute) bool {
		for _, x := range chosen {
			if x == r {
				return true
			}
		}
		return false
	}
	requested := map[string]bool{}
	var found []string
	for _, n := range names {
		if strings.Contains(n, query) || matches(query, n) {
			found = append(found, n)
		}
	}
	for _, n := range found {
		out = append(out, "## front "+n)
		for _, r := range mods[n].routes {
			out = append(out, fmt.Sprintf("route %s -> %s  %s", r.path, r.view, filepath.ToSlash(filepath.Join(frontRel, r.at))))
		}
		for _, call := range mods[n].calls {
			head, _, _ := strings.Cut(call, "  ")
			_, url, _ := strings.Cut(head, " ")
			requested[callKey(url)] = true
		}
	}
	covered := map[string]bool{}
	for _, r := range routes {
		if len(found) > 0 && requested[r.uri] || len(found) == 0 && (strings.Contains(r.uri, query) || matches(query, r.uri)) {
			chosen = append(chosen, r)
			covered[r.uri] = true
		}
	}
	for _, r := range routes {
		if !in(r) && (matches(query, r.uri) || matches(query, backendModule(r.action))) {
			chosen = append(chosen, r)
		}
	}
	out = append(out, "## backend")
	head, apis := out, []string{}
	out = nil
	var controllers, services, models []string
	add := func(list *[]string, v string) {
		if v != "" && !slicesContains(*list, v) {
			*list = append(*list, v)
		}
	}
	relOf := func(p string) string { r, _ := filepath.Rel(root, p); return filepath.ToSlash(r) }
	for _, r := range chosen {
		cls, method, _ := strings.Cut(r.action, "@")
		line := ""
		if f := fileOf(root, cls); f != "" {
			add(&controllers, f)
			n := 0
			if raw, err := os.ReadFile(f); err == nil {
				for i, l := range strings.Split(string(raw), "\n") {
					if strings.Contains(l, "function "+method+"(") {
						n = i + 1
						break
					}
				}
			}
			line = fmt.Sprintf("  %s:%d", relOf(f), n)
		}
		cross := calls[r.method+" "+r.uri]
		if len(cross) > 2 {
			cross = cross[:2]
		}
		l := fmt.Sprintf("api %s %s -> %s@%s%s", r.method, r.uri, cls[strings.LastIndex(cls, `\`)+1:], method, line)
		if len(cross) > 0 {
			l += "  <- " + strings.Join(cross, ", ")
		}
		apis = append(apis, l)
	}
	var orphan []string
	for u := range requested {
		if !covered[u] {
			orphan = append(orphan, u)
		}
	}
	sort.Strings(orphan)
	for _, u := range orphan {
		out = append(out, "api without a route in the backend: "+u)
	}
	for _, f := range controllers {
		for _, s := range importsOf(f, "Services") {
			add(&services, fileOf(root, s))
		}
		for _, m := range importsOf(f, "Models") {
			add(&models, fileOf(root, m))
		}
	}
	for _, f := range services {
		for _, m := range importsOf(f, "Models") {
			add(&models, fileOf(root, m))
		}
	}
	for _, f := range services {
		out = append(out, "service "+relOf(f))
	}
	for _, f := range models {
		t := "?"
		if raw, err := os.ReadFile(f); err == nil {
			if m := tableRe.FindStringSubmatch(string(raw)); m != nil {
				t = m[1]
			}
		}
		out = append(out, fmt.Sprintf("model %s -> table %s", relOf(f), t))
	}
	if len(chosen) == 0 && len(found) == 0 {
		return fmt.Sprintf("no module or route contains «%s». Next: query is the path of a front module (crm/quotes) or words of an API address (quote approve); the names in the code are in English, so a query in another language finds nothing", echo(query)), nil
	}
	// The endpoints are the long part: when they do not fit, the last ones go and the services and tables stay.
	room := maxBytes - len(strings.Join(head, "\n")) - len(strings.Join(out, "\n")) - 120
	var shown []string
	for _, l := range apis {
		if room -= len(l) + 1; room < 0 {
			break
		}
		shown = append(shown, l)
	}
	if len(shown) < len(apis) {
		shown = append(shown, fmt.Sprintf("(… and %d more endpoints not shown: narrow the query)", len(apis)-len(shown)))
	}
	return capOut(strings.Join(append(append(head, shown...), out...), "\n")), nil
}
