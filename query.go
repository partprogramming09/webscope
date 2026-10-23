package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

const (
	defaultLimit = 50
	maxLimit     = 500
	maxCell      = 200  // runes in one cell
	maxBytes     = 8000 // bytes of rows in one answer
	queryTimeout = 15 * time.Second
)

// source is the database named by WEBSCOPE_DB_URL, opened the first time a query needs it.
type source struct {
	url    string
	mu     sync.Mutex
	db     *sql.DB
	driver string
}

type queryArgs struct {
	SQL   string `json:"sql" jsonschema:"one read-only statement: SELECT, SHOW, DESCRIBE, EXPLAIN or WITH"`
	Limit int    `json:"limit,omitempty" jsonschema:"rows to show, default 50, at most 500"`
}

func (s *source) conn() (*sql.DB, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		db, driver, err := openDB(s.url)
		if err != nil {
			return nil, "", err
		}
		s.db, s.driver = db, driver
	}
	return s.db, s.driver, nil
}

// answer checks the statement, runs it and returns the text for the agent; no error text carries the password of the URL.
func (s *source) answer(ctx context.Context, in queryArgs) (string, error) {
	pw := password(s.url)
	fail := func(err error) (string, error) { return "", errors.New(redact(err.Error(), pw)) }
	stmt, err := checkSQL(in.SQL)
	if err != nil {
		return fail(err)
	}
	db, driver, err := s.conn()
	if err != nil {
		return fail(err)
	}
	out, err := runQuery(ctx, db, driver, stmt, rowLimit(in.Limit))
	if err != nil {
		return fail(err)
	}
	return out, nil
}

// openDB opens mysql://user:password@host:port/db or sqlite:///path/to/file.db. The SQLite file is opened with query_only on.
func openDB(raw string) (*sql.DB, string, error) {
	if raw == "" {
		return nil, "", errors.New("WEBSCOPE_DB_URL is not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, "", errors.New("WEBSCOPE_DB_URL is not a valid URL")
	}
	switch u.Scheme {
	case "mysql":
		cfg := mysql.NewConfig()
		cfg.User = u.User.Username()
		cfg.Passwd, _ = u.User.Password()
		cfg.Net, cfg.Addr, cfg.DBName = "tcp", u.Host, strings.TrimPrefix(u.Path, "/")
		cfg.Timeout, cfg.ReadTimeout = 5*time.Second, queryTimeout
		c, err := mysql.NewConnector(cfg)
		if err != nil {
			return nil, "", err
		}
		return sql.OpenDB(c), "mysql", nil
	case "sqlite":
		db, err := sql.Open("sqlite", "file:"+u.Host+u.Path+"?_pragma=query_only(1)")
		return db, "sqlite", err
	}
	return nil, "", fmt.Errorf("unsupported database URL scheme %q: use mysql or sqlite", u.Scheme)
}

// password is the password inside a database URL, or "".
func password(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.User != nil {
		pw, _ := u.User.Password()
		return pw
	}
	return ""
}

// redact hides every occurrence of secret in msg.
func redact(msg, secret string) string {
	if secret == "" {
		return msg
	}
	return strings.ReplaceAll(msg, secret, "***")
}

// rowLimit is the number of rows to show: the default when n is not positive, never more than maxLimit.
func rowLimit(n int) int {
	switch {
	case n <= 0:
		return defaultLimit
	case n > maxLimit:
		return maxLimit
	}
	return n
}

// runQuery runs one checked statement in a read-only transaction that is always rolled back (the second layer after checkSQL: on MySQL the
// transaction is READ ONLY, on SQLite the file is opened with query_only) and returns a TSV table with a header and a closing line that says
// whether rows were left out.
func runQuery(ctx context.Context, db *sql.DB, driver, stmt string, limit int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: driver == "mysql"})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, stmt)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(strings.Join(cols, "\t") + "\n")
	vals, ptrs := make([]any, len(cols)), make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	n, more, cut := 0, false, false
	for rows.Next() {
		if n == limit {
			more = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", err
		}
		cells := make([]string, len(cols))
		for i, v := range vals {
			cells[i] = cell(v)
		}
		row := strings.Join(cells, "\t") + "\n"
		if n > 0 && b.Len()+len(row) > maxBytes {
			cut = true
			break
		}
		b.WriteString(row)
		n++
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	switch {
	case cut:
		fmt.Fprintf(&b, "(%d rows shown, cut at %d bytes: select fewer columns or add WHERE)", n, maxBytes)
	case more:
		fmt.Fprintf(&b, "(%d rows shown, more exist: add WHERE or select fewer rows)", n)
	default:
		fmt.Fprintf(&b, "(%d rows)", n)
	}
	return b.String(), nil
}

// cell is one value as text: \N for NULL, no tabs or line breaks, at most maxCell runes.
func cell(v any) string {
	var s string
	switch x := v.(type) {
	case nil:
		return `\N`
	case []byte:
		if !utf8.Valid(x) {
			return fmt.Sprintf("<binary %d bytes>", len(x))
		}
		s = string(x)
	case time.Time:
		s = x.Format("2006-01-02 15:04:05")
	default:
		s = fmt.Sprint(x)
	}
	s = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
	if r := []rune(s); len(r) > maxCell {
		s = string(r[:maxCell]) + "…"
	}
	return s
}
