package code

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/partprogramming09/webscope/internal/fsx"
)

// sent is a field that a front call sends; optional when its TypeScript type marks it with ? or only some members of a union have it.
type sent struct {
	name     string
	optional bool
}

var (
	writeCall  = regexp.MustCompile(`\b(?:axios|http|api)\.(?:post|put|patch)(?:<(?:[^<>]|<(?:[^<>]|<[^<>]*>)*>)*>)?\(`)
	identRe    = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
	entryKey   = regexp.MustCompile(`^(?:readonly\s+)?(?:'([^']+)'|"([^"]+)"|([A-Za-z_$][\w$]*))\s*(\?)?\s*:`)
	shorthand  = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
	splitWords = []*regexp.Regexp{regexp.MustCompile(`([a-z0-9])([A-Z])`), regexp.MustCompile(`([A-Z])([A-Z][a-z])`)}
	notAlnum   = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

// sentFields reads the body of the post, put or patch call at file:line, with the types of the .ts files under typeRoot.
func sentFields(typeRoot, file string, line int) ([]sent, []string) {
	return sentFieldsIn(&typeIndex{root: typeRoot}, file, line)
}

// sentFieldsIn reads the body of the call at file:line: an object literal, a variable holding one, or a typed parameter; what cannot be read
// is named in the second result.
func sentFieldsIn(idx *typeIndex, file string, line int) (fields []sent, unresolved []string) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, []string{"the file could not be read"}
	}
	text := blankComments(string(raw), false)
	from := lineOffset(text, line)
	loc := writeCall.FindStringIndex(text[from:])
	if loc == nil || from+loc[0] > lineEnd(text, from) {
		return nil, []string{"no post, put or patch call on that line"}
	}
	open := from + loc[1] - 1
	end := closingIn(text, open, false)
	if end < 0 {
		return nil, []string{"the call does not close"}
	}
	args := splitTop(text[open+1:end], false, ',')
	if len(args) < 2 {
		return nil, nil // a call with no body
	}
	r := &frontReader{idx: idx, file: file, before: text[:open], visiting: map[string]bool{}}
	return r.expr(strings.TrimSpace(args[1]), false)
}

// frontReader reads one call: before is the file text up to it.
type frontReader struct {
	idx          *typeIndex
	file, before string
	visiting     map[string]bool // the variables being read: a spread that comes back to one is a cycle
	memo         map[string]typeRead
}

type typeRead struct {
	fields []sent
	ok     bool
}

// expr reads the fields of a body expression; spread is true when it comes after ... in an object.
func (r *frontReader) expr(e string, spread bool) ([]sent, []string) {
	label := e
	if spread {
		label = "..." + e
	}
	switch {
	case strings.HasPrefix(e, "{"):
		return r.object(e)
	case !identRe.MatchString(e):
		return nil, []string{label + " (an expression: fields not read)"}
	case r.visiting[e]:
		return nil, []string{label + " (spreads come back to it: fields not read)"}
	}
	r.visiting[e] = true
	defer delete(r.visiting, e)
	obj, objAt := r.assignedObject(e)
	t, typeAt := r.paramType(e)
	if obj != "" && objAt > typeAt {
		return r.object(obj)
	}
	switch {
	case t == "FormData" || regexp.MustCompile(`\b`+regexp.QuoteMeta(e)+`\s*=\s*new\s+FormData\b`).MatchString(r.before):
		return nil, []string{label + " (FormData: fields not read)"}
	case t == "":
		return nil, []string{label + " (no type)"}
	}
	fs, ok := r.typeFields(t, map[string]bool{})
	if !ok {
		return fs, []string{label + " (type " + t + " not found, or declared in more than one place)"}
	}
	return fs, nil
}

// object reads the first-level keys of an object literal {…}; a spread is read as its own expression.
func (r *frontReader) object(lit string) (out []sent, unresolved []string) {
	lit = strings.TrimSpace(lit)
	end := closingIn(lit, 0, false)
	if end < 1 {
		return nil, []string{"an object that does not close"}
	}
	for _, part := range splitTop(lit[1:end], false, ',') {
		part = strings.TrimSpace(part)
		switch {
		case part == "":
		case strings.HasPrefix(part, "..."):
			fs, un := r.expr(strings.TrimSpace(part[3:]), true)
			out, unresolved = append(out, fs...), append(unresolved, un...)
		default:
			if m := entryKey.FindStringSubmatch(part); m != nil {
				out = append(out, sent{name: m[1] + m[2] + m[3]})
			} else if shorthand.MatchString(part) {
				out = append(out, sent{name: part})
			}
		}
	}
	return mergeSent(out), unresolved
}

// assignedObject is the object literal last assigned to name (const, let or var) before the call and where it starts, or "" and -1.
func (r *frontReader) assignedObject(name string) (string, int) {
	locs := regexp.MustCompile(`\b(?:const|let|var)\s+`+regexp.QuoteMeta(name)+`\s*(?::[^=]+)?=\s*\{`).FindAllStringIndex(r.before, -1)
	if len(locs) == 0 {
		return "", -1
	}
	open := locs[len(locs)-1][1] - 1
	end := closingIn(r.before, open, false)
	if end < 0 {
		return "", -1
	}
	return r.before[open : end+1], open
}

// paramType is the type written after `name:` closest before the call and where it is, or "" and -1.
func (r *frontReader) paramType(name string) (string, int) {
	locs := regexp.MustCompile(`[(,]\s*`+regexp.QuoteMeta(name)+`\s*\??\s*:\s*`).FindAllStringIndex(r.before, -1)
	if len(locs) == 0 {
		return "", -1
	}
	at := locs[len(locs)-1][1]
	depth := 0
	for i := at; i < len(r.before); i++ {
		switch r.before[i] {
		case '<', '(', '{', '[':
			depth++
		case '>', ')', '}', ']':
			if depth == 0 {
				return strings.TrimSpace(r.before[at:i]), at
			}
			depth--
		case ',', '=':
			if depth == 0 {
				return strings.TrimSpace(r.before[at:i]), at
			}
		}
	}
	return "", -1
}

var (
	typeDeclRe  = regexp.MustCompile(`(?m)^\s*(?:export\s+)?type\s+(\w+)\s*=\s*`)
	ifaceDeclRe = regexp.MustCompile(`(?m)^\s*(?:export\s+)?interface\s+(\w+)\s*(?:extends\s+([\w\s,]+?))?\s*\{`)
)

// typeIndex holds the type and interface declarations of the .ts files under root, read once on first use.
type typeIndex struct {
	root   string
	loaded bool
	decls  map[string][]typeDecl
}

// typeDecl is one declaration: the file that holds it and its definition ("= …", an interface as its body & its extends).
type typeDecl struct{ file, def string }

func (x *typeIndex) load() {
	x.loaded, x.decls = true, map[string][]typeDecl{}
	files, _, err := fsx.WalkFiles(x.root, func(n string) bool { return strings.HasSuffix(n, ".ts") })
	if err != nil {
		return
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := blankComments(string(raw), false)
		for _, m := range typeDeclRe.FindAllStringSubmatchIndex(text, -1) {
			name := text[m[2]:m[3]]
			x.decls[name] = append(x.decls[name], typeDecl{f, "=" + text[m[1]:typeEnd(text, m[1])]})
		}
		for _, m := range ifaceDeclRe.FindAllStringSubmatchIndex(text, -1) {
			name, open := text[m[2]:m[3]], m[1]-1
			end := closingIn(text, open, false)
			if end < 0 {
				continue
			}
			def := text[open : end+1]
			if m[4] >= 0 {
				for _, ext := range strings.Split(text[m[4]:m[5]], ",") {
					def += " & " + strings.TrimSpace(ext)
				}
			}
			x.decls[name] = append(x.decls[name], typeDecl{f, "=" + def})
		}
	}
}

// lookup is the declaration of name closest to the file that uses it (the most folders in common); two equally close ones are ambiguous.
func (x *typeIndex) lookup(name, from string) (string, bool) {
	if !x.loaded {
		x.load()
	}
	best, bestLen, tie := "", -1, false
	for _, d := range x.decls[name] {
		n := commonDirs(filepath.Dir(d.file), filepath.Dir(from))
		switch {
		case n > bestLen:
			best, bestLen, tie = d.def, n, false
		case n == bestLen && d.def != best:
			tie = true
		}
	}
	return best, bestLen >= 0 && !tie
}

// commonDirs is how many leading folders a and b share.
func commonDirs(a, b string) int {
	x, y := strings.Split(filepath.ToSlash(a), "/"), strings.Split(filepath.ToSlash(b), "/")
	n := 0
	for n < len(x) && n < len(y) && x[n] == y[n] {
		n++
	}
	return n
}

// typeEnd is where a type alias that starts at i ends: a ; at its level, or a line end at its level not followed or preceded by | or &.
func typeEnd(text string, i int) int {
	depth := 0
	for j := i; j < len(text); j++ {
		switch c := text[j]; c {
		case '\'', '"', '`':
			j = skipString(text, j)
		case '{', '(', '[', '<':
			depth++
		case '}', ')', ']', '>':
			if c == '>' && j > i && text[j-1] == '=' { // => is not a bracket
				continue
			}
			depth--
		case ';':
			if depth == 0 {
				return j
			}
		case '\n':
			if depth != 0 {
				continue
			}
			prev := strings.TrimRight(text[i:j], " \t\r")
			next := strings.TrimLeft(text[j:], " \t\r\n")
			if strings.HasSuffix(prev, "|") || strings.HasSuffix(prev, "&") || strings.HasSuffix(prev, "=") || strings.HasPrefix(next, "|") || strings.HasPrefix(next, "&") {
				continue
			}
			return j
		}
	}
	return len(text)
}

// typeFields reads the fields of a type expression: literals, names of declared types, & (all fields) and | (a field missing in a member is optional).
func (r *frontReader) typeFields(t string, seen map[string]bool) ([]sent, bool) {
	t = strings.TrimSpace(t)
	members := splitTop(t, true, '|')
	if len(members) > 1 {
		count, opt := map[string]int{}, map[string]bool{}
		var all []sent
		ok := true
		for _, m := range members {
			if strings.TrimSpace(m) == "" || strings.TrimSpace(m) == "null" || strings.TrimSpace(m) == "undefined" {
				continue
			}
			fs, k := r.typeFields(m, seen)
			ok = ok && k
			for _, f := range fs {
				count[f.name]++
				opt[f.name] = opt[f.name] || f.optional
			}
			all = append(all, fs...)
		}
		real := 0
		for _, m := range members {
			if s := strings.TrimSpace(m); s != "" && s != "null" && s != "undefined" {
				real++
			}
		}
		merged := mergeSent(all)
		for i := range merged {
			if count[merged[i].name] < real || opt[merged[i].name] {
				merged[i].optional = true
			}
		}
		return merged, ok
	}
	var out []sent
	ok := true
	for _, part := range splitTop(t, true, '&') {
		part = strings.TrimSpace(part)
		switch {
		case part == "":
		case strings.HasPrefix(part, "{"):
			out = append(out, literalTypeFields(part)...)
		case identRe.MatchString(part):
			if m, done := r.memo[part]; done {
				out, ok = append(out, m.fields...), ok && m.ok
				continue
			}
			def, found := r.idx.lookup(part, r.file)
			if !found || seen[part] {
				ok = false
				continue
			}
			seen[part] = true
			fs, k := r.typeFields(strings.TrimPrefix(def, "="), seen)
			delete(seen, part)
			if r.memo == nil {
				r.memo = map[string]typeRead{}
			}
			if k { // a read cut by a cycle is partial: not kept
				r.memo[part] = typeRead{fs, k}
			}
			out, ok = append(out, fs...), ok && k
		default:
			ok = false // Partial<X>, Omit<…>, X[]: not read
		}
	}
	return mergeSent(out), ok
}

// literalTypeFields reads the first-level fields of a type literal { a: string; b?: number }.
func literalTypeFields(lit string) []sent {
	end := closingIn(lit, 0, false)
	if end < 0 {
		return nil
	}
	var out []sent
	for _, line := range splitTop(lit[1:end], true, ';', ',', '\n') {
		if m := entryKey.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			out = append(out, sent{name: m[1] + m[2] + m[3], optional: m[4] == "?"})
		}
	}
	return out
}

// mergeSent keeps one entry per name, in first order; a name is optional only if every entry says so.
func mergeSent(fs []sent) []sent {
	at := map[string]int{}
	var out []sent
	for _, f := range fs {
		if i, ok := at[f.name]; ok {
			out[i].optional = out[i].optional && f.optional
			continue
		}
		at[f.name] = len(out)
		out = append(out, f)
	}
	return out
}

// splitTop splits s at any of seps that is outside brackets and strings; < and > count as brackets only in a type (angles).
func splitTop(s string, angles bool, seps ...byte) []string {
	var out []string
	depth, from := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\'', '"', '`':
			i = skipString(s, i)
		case '{', '(', '[', '<':
			if c == '<' && !angles {
				continue
			}
			depth++
		case '}', ')', ']', '>':
			if c == '>' && (!angles || i > 0 && s[i-1] == '=') { // => is not a bracket
				continue
			}
			depth--
		case '/':
			if i+1 < len(s) && s[i+1] == '/' {
				i = lineEnd(s, i) - 1 // the newline stays: it may be a separator
			}
		default:
			if depth == 0 && strings.IndexByte(string(seps), c) >= 0 {
				out = append(out, s[from:i])
				from = i + 1
			}
		}
	}
	return append(out, s[from:])
}

// wireName is the name as the backend receives it: snake_case like axios-case-converter (snake-case 3) when snake, else as written.
func wireName(s string, snake bool) string {
	if !snake {
		return s
	}
	for _, re := range splitWords {
		s = re.ReplaceAllString(s, "${1}_${2}")
	}
	return strings.Trim(strings.ToLower(notAlnum.ReplaceAllString(s, "_")), "_")
}

// caseConverter reports whether the package.json of the front has axios-case-converter as a dependency.
func caseConverter(frontRoot string) bool {
	raw, err := os.ReadFile(filepath.Join(frontRoot, "package.json"))
	if err != nil {
		return false
	}
	var pkg map[string]json.RawMessage // a map, not a struct: the keys must match exactly, as npm reads them
	if json.Unmarshal([]byte(strings.TrimPrefix(string(raw), "\ufeff")), &pkg) != nil {
		return false
	}
	for _, k := range []string{"dependencies", "devDependencies"} {
		var deps map[string]any
		if json.Unmarshal(pkg[k], &deps) == nil {
			if _, ok := deps["axios-case-converter"]; ok {
				return true
			}
		}
	}
	return false
}

func lineOffset(text string, line int) int {
	at := 0
	for n := 1; n < line; n++ {
		i := strings.IndexByte(text[at:], '\n')
		if i < 0 {
			return len(text)
		}
		at += i + 1
	}
	return at
}
