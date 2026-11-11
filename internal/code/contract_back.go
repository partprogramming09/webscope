package code

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// rule is one key of the validation rules of a backend method; a nested key (items.*.qty) counts under its first part.
type rule struct {
	name               string
	required, nullable bool
	line               int
}

// ruleSet is what a method validates: source is the FormRequest class, "inline validate", or "" when the method validates nothing that was found.
// literal is false when the rules are not a plain [ … ] (array_merge, parent::rules(), a + b): then rules may be short.
type ruleSet struct {
	source, file string
	rules        []rule
	literal      bool
}

var (
	requestParam = regexp.MustCompile(`\b([A-Z]\w*Request)\s+\$\w+`)
	ruleKey      = regexp.MustCompile(`['"]([^'"]+)['"]\s*=>`)
	requiredWord = regexp.MustCompile(`\brequired\b`)
	nullableWord = regexp.MustCompile(`\bnullable\b`)
	inlineCall   = regexp.MustCompile(`(?:\bvalidate|\bValidator::make)\([^\[;]*\[`)
	sometimesRe  = regexp.MustCompile(`\bsometimes\b`)
)

// rulesFor reads the validation rules of method in controllerFile: the FormRequest of its parameters (found by its use line, or in the namespace
// of the controller) or a validate([…]) inside its body. root is the real path of the backend; a request outside it is never read.
func rulesFor(root, controllerFile, method string) (ruleSet, error) {
	raw, err := os.ReadFile(controllerFile)
	if err != nil {
		return ruleSet{}, err
	}
	text := blankComments(string(raw), true)
	sig, body, bodyAt, ok := methodSpan(text, method)
	if !ok {
		return ruleSet{literal: true}, nil
	}
	var missing string // a FormRequest the signature names whose file is not under the backend
	if m := requestParam.FindStringSubmatch(sig); m != nil {
		if f := fileOf(root, classOf(text, m[1])); f != "" {
			req, err := os.ReadFile(f)
			if err != nil {
				return ruleSet{}, err
			}
			rs := rulesOfRequest(blankComments(string(req), true))
			rs.source, rs.file = m[1], f
			return rs, nil
		}
		missing = m[1]
	}
	if loc := inlineCall.FindStringIndex(body); loc != nil {
		open := bodyAt + loc[1] - 1
		end := closing(text, open)
		if end < 0 {
			return ruleSet{source: "inline validate", file: controllerFile}, nil
		}
		return ruleSet{source: "inline validate", file: controllerFile, rules: keysOf(text, open, end), literal: true}, nil
	}
	if missing != "" { // its rules are unknown, not absent
		return ruleSet{source: missing}, nil
	}
	return ruleSet{literal: true}, nil
}

// classOf is the full name of the class short as the PHP file text sees it: its use line (also `use X as short`), else the namespace of the file.
func classOf(text, short string) string {
	for _, m := range regexp.MustCompile(`(?m)^use\s+([\w\\]+?)(?:\s+as\s+(\w+))?\s*;`).FindAllStringSubmatch(text, -1) {
		last := m[1][strings.LastIndex(m[1], `\`)+1:]
		if m[2] == short || m[2] == "" && last == short {
			return m[1]
		}
	}
	if ns := regexp.MustCompile(`(?m)^namespace\s+([\w\\]+)\s*;`).FindStringSubmatch(text); ns != nil {
		return ns[1] + `\` + short
	}
	return short
}

// rulesOfRequest reads the keys of the array that rules() returns.
func rulesOfRequest(text string) ruleSet {
	_, body, bodyAt, ok := methodSpan(text, "rules")
	if !ok {
		return ruleSet{}
	}
	ret := regexp.MustCompile(`\breturn\b\s*`).FindStringIndex(body)
	if ret == nil {
		return ruleSet{}
	}
	open := bodyAt + ret[1]
	if open >= len(text) || text[open] != '[' {
		return ruleSet{}
	}
	end := closing(text, open)
	if end < 0 {
		return ruleSet{}
	}
	literal := strings.HasPrefix(strings.TrimLeft(text[end+1:], " \t\r\n"), ";")
	return ruleSet{rules: keysOf(text, open, end), literal: literal}
}

// keysOf reads the 'key' => value pairs at the first level of the PHP array text[open:end+1].
func keysOf(text string, open, end int) []rule {
	var out []rule
	at := map[string]int{}
	depth := 0
	for i := open; i <= end; i++ {
		switch c := text[i]; c {
		case '\'', '"':
			j := skipString(text, i)
			if depth == 1 {
				if m := ruleKey.FindStringSubmatchIndex(text[i:min(end, i+300)]); m != nil && m[0] == 0 {
					key := text[i+m[2] : i+m[3]]
					vstart := i + m[1]
					vend := valueEnd(text, vstart, end)
					value := text[vstart:vend]
					name, child := key, false
					if k, _, nested := strings.Cut(key, "."); nested {
						name, child = k, true
					}
					idx, seen := at[name]
					if !seen {
						at[name] = len(out)
						out = append(out, rule{name: name, line: lineAt(text, i)})
						idx = len(out) - 1
					}
					if !child {
						out[idx].required = requiredWord.MatchString(value) && !sometimesRe.MatchString(value)
						out[idx].nullable = nullableWord.MatchString(value)
					}
					i = vend - 1
					continue
				}
			}
			i = j
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			depth--
		case '/':
			if i+1 <= end && text[i+1] == '/' {
				i = lineEnd(text, i)
			}
		case '#':
			i = lineEnd(text, i)
		}
	}
	return out
}

// valueEnd is where the value that starts at i ends: the next comma at its own level, or end.
func valueEnd(text string, i, end int) int {
	depth := 0
	for ; i < end; i++ {
		switch text[i] {
		case '\'', '"':
			i = skipString(text, i)
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			if depth == 0 {
				return i
			}
			depth--
		case ',':
			if depth == 0 {
				return i
			}
		}
	}
	return end
}

// methodSpan finds `function name(` in text: its signature (up to the {), its body, and where the body starts in text.
func methodSpan(text, name string) (sig, body string, bodyAt int, ok bool) {
	loc := regexp.MustCompile(`\bfunction\s+` + regexp.QuoteMeta(name) + `\s*\(`).FindStringIndex(text)
	if loc == nil {
		return "", "", 0, false
	}
	brace := -1
	for i := loc[1]; i < len(text); i++ {
		if text[i] == '\'' || text[i] == '"' {
			i = skipString(text, i)
			continue
		}
		if text[i] == ';' { // an abstract method or an interface: no body
			return "", "", 0, false
		}
		if text[i] == '{' {
			brace = i
			break
		}
	}
	if brace < 0 {
		return "", "", 0, false
	}
	end := closing(text, brace)
	if end < 0 {
		end = len(text) - 1
	}
	return text[loc[0]:brace], text[brace : end+1], brace, true
}

// closing is the index of the bracket that closes the one at open ([, ( or {) in PHP, skipping strings and comments; -1 if it never closes.
func closing(text string, open int) int { return closingIn(text, open, true) }

// closingIn is closing for PHP (# starts a comment) or TypeScript (it does not).
func closingIn(text string, open int, php bool) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '\'', '"', '`':
			i = skipString(text, i)
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			if depth--; depth == 0 {
				return i
			}
		case '/':
			if i+1 < len(text) && text[i+1] == '/' {
				i = lineEnd(text, i)
			} else if i+1 < len(text) && text[i+1] == '*' {
				if j := strings.Index(text[i+2:], "*/"); j >= 0 {
					i += j + 3
				} else {
					return -1
				}
			}
		case '#':
			if php && i+1 < len(text) && text[i+1] != '[' { // #[Attribute] is code
				i = lineEnd(text, i)
			}
		}
	}
	return -1
}

// skipString is the index of the quote that closes the string opened at i (backslash escapes), or the last index of text.
func skipString(text string, i int) int {
	q := text[i]
	for j := i + 1; j < len(text); j++ {
		switch text[j] {
		case '\\':
			j++
		case q:
			return j
		}
	}
	return len(text) - 1
}

func lineEnd(text string, i int) int {
	if j := strings.IndexByte(text[i:], '\n'); j >= 0 {
		return i + j
	}
	return len(text) - 1
}

// relTo is p relative to root with forward slashes, or p when it is not under root.
func relTo(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}

var heredocStart = regexp.MustCompile(`^<<<[ \t]*['"]?([A-Za-z_]\w*)['"]?\r?\n`)

// blankComments turns comments (and, in PHP, heredocs) into spaces, keeping every newline, so offsets and line numbers stay the same.
func blankComments(text string, php bool) string {
	b := []byte(text)
	blank := func(from, to int) {
		for k := from; k < to && k < len(b); k++ {
			if b[k] != '\n' {
				b[k] = ' '
			}
		}
	}
	for i := 0; i < len(b); i++ {
		switch c := b[i]; {
		case c == '\'' || c == '"' || c == '`' && !php:
			i = skipString(text, i)
		case c == '/' && i+1 < len(b) && b[i+1] == '/', php && c == '#' && (i+1 >= len(b) || b[i+1] != '['):
			end := lineEnd(text, i)
			if end < len(b) && b[end] != '\n' { // the last line has no newline: blank it to the end
				end = len(b)
			}
			blank(i, end)
			i = end
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				blank(i, len(b))
				return string(b)
			}
			blank(i, i+2+end+2)
			i += end + 3
		case php && c == '<' && strings.HasPrefix(text[i:], "<<<"):
			m := heredocStart.FindStringSubmatch(text[i:])
			if m == nil {
				continue
			}
			close := regexp.MustCompile(`(?m)^[ \t]*` + m[1] + `\b`).FindStringIndex(text[i+len(m[0]):])
			end := len(b)
			if close != nil {
				end = i + len(m[0]) + close[1]
			}
			blank(i, end)
			b[i], b[i+1] = '\'', '\'' // the heredoc stays a value: an empty string
			i = end - 1
		}
	}
	return string(b)
}
