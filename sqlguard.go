package main

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// allowed are the words a read-only statement may start with.
var allowed = map[string]bool{"SELECT": true, "SHOW": true, "DESCRIBE": true, "DESC": true, "EXPLAIN": true, "WITH": true}

// checkSQL returns stmt without trailing whitespace and semicolons when it is one statement that starts with an allowed word and has no INTO
// outside quotes and comments. It is the first of two layers: the database session is read-only as well (see runQuery), which also stops a
// write that starts with WITH, since this check cannot tell it apart from a read.
func checkSQL(stmt string) (string, error) {
	// The text is read under the rules of both engines (MySQL escapes with a backslash, SQLite has [bracket] identifiers and no backslash escape):
	// it must be one safe read under each, because the guard does not know which engine will run it.
	out, err := checkDialect(stmt, false)
	if err != nil {
		return "", err
	}
	if _, err := checkDialect(stmt, true); err != nil {
		return "", err
	}
	return out, nil
}

func checkDialect(stmt string, sqlite bool) (string, error) {
	code, err := mask(stmt, sqlite)
	if err != nil {
		return "", err
	}
	end := len(code)
	for end > 0 && (code[end-1] == ';' || code[end-1] == ' ' || code[end-1] == '\n' || code[end-1] == '\t' || code[end-1] == '\r') {
		end--
	}
	code, stmt = code[:end], stmt[:end]
	if strings.TrimSpace(code) == "" {
		return "", errors.New("empty statement")
	}
	if strings.Contains(code, ";") {
		return "", errors.New("one statement only: remove the inner ';'")
	}
	if first := firstWord(code); !allowed[strings.ToUpper(first)] {
		return "", fmt.Errorf("the statement must start with SELECT, SHOW, DESCRIBE, DESC, EXPLAIN or WITH, not %q", first)
	}
	for _, w := range strings.FieldsFunc(code, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$' // '@' separates: INTO@x is INTO followed by a variable
	}) {
		if strings.EqualFold(w, "INTO") {
			return "", errors.New("INTO writes files or variables")
		}
		if strings.EqualFold(w, "LOAD_FILE") || strings.EqualFold(w, "GET_LOCK") {
			return "", fmt.Errorf("%s reads a file of the server or takes a lock that others wait for", strings.ToUpper(w))
		}
	}
	return strings.TrimSpace(stmt), nil
}

// firstWord is the leading run of letters of code, or its first character when it starts with anything else.
func firstWord(code string) string {
	code = strings.TrimLeft(code, " \t\r\n")
	n := 0
	for n < len(code) && (code[n] >= 'a' && code[n] <= 'z' || code[n] >= 'A' && code[n] <= 'Z') {
		n++
	}
	if n == 0 && code != "" {
		return code[:1]
	}
	return code[:n]
}

// mask returns s with the inside of quoted text and the comments replaced by spaces, byte for byte, so a ';' or a verb in a string or comment
// is not read as code. It refuses an unterminated quote or comment, and the executable comments /*! ... */ that MySQL runs as code.
// With sqlite the rules are SQLite's: [bracket] identifiers, and no backslash escapes. checkSQL reads a statement under both.
func mask(s string, sqlite bool) (string, error) {
	b := []byte(s)
	blank := func(from, to int) {
		for k := from; k < to; k++ {
			b[k] = ' '
		}
	}
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for ; j < len(b); j++ {
				if b[j] == '\\' && c != '`' && !sqlite {
					j++
					continue
				}
				if b[j] == c {
					if j+1 < len(b) && b[j+1] == c {
						j++
						continue
					}
					break
				}
			}
			if j >= len(b) {
				return "", errors.New("unterminated quote")
			}
			blank(i+1, j)
			i = j + 1
		case c == '[' && sqlite: // a SQLite identifier: nothing inside it is code
			j := i + 1
			for j < len(b) && b[j] != ']' {
				j++
			}
			if j >= len(b) {
				return "", errors.New("unterminated [identifier]")
			}
			blank(i+1, j)
			i = j + 1
		case c == '#' || (c == '-' && i+1 < len(b) && b[i+1] == '-' && (i+2 >= len(b) || strings.IndexByte(" \t\r\n", b[i+2]) >= 0)):
			j := i
			for j < len(b) && b[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			if i+2 < len(b) && (b[i+2] == '!' || (b[i+2] == 'M' && i+3 < len(b) && b[i+3] == '!')) {
				return "", errors.New("executable comment /*! ... */ is not allowed")
			}
			end := strings.Index(string(b[i+2:]), "*/")
			if end < 0 {
				return "", errors.New("unterminated comment")
			}
			blank(i, i+2+end+2)
			i += 2 + end + 2
		default:
			i++
		}
	}
	return string(b), nil
}
