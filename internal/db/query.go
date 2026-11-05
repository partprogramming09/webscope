package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/partprogramming09/webscope/internal/input"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
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
	maxCell      = 200 // runes in one cell
	QueryTimeout = 15 * time.Second
)

// Source is the database named by WEBSCOPE_DB_URL, opened the first time a query needs it.
type Source struct {
	URL         string
	TenantQuery string
	mu          sync.Mutex
	dbs         map[string]*sql.DB // by database name; "" is the one in the URL
	driver      string
}

// QueryArgs are the arguments of the query tool.
type QueryArgs struct {
	SQL    string `json:"sql" jsonschema:"one read-only statement"`
	Tenant string `json:"tenant,omitempty" jsonschema:"tenant fqdn; omit for the URL's database"`
	Limit  int    `json:"limit,omitempty" jsonschema:"rows, default 50, max 500"`
}

// HynTenantQuery is the default way to find a tenant's database: Hyn Tenancy keeps it in websites.uuid. WEBSCOPE_TENANT_QUERY replaces it.
const HynTenantQuery = `SELECT w.uuid, w.managed_by_database_connection FROM hostnames h JOIN websites w ON w.id = h.website_id WHERE h.fqdn = ? AND h.deleted_at IS NULL AND w.deleted_at IS NULL`

// conn returns the connection to database, opened the first time; "" is the database in the URL.
func (s *Source) conn(database string) (*sql.DB, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if db, ok := s.dbs[database]; ok {
		return db, s.driver, nil
	}
	db, driver, err := openDB(s.URL, database)
	if err != nil {
		return nil, "", err
	}
	if s.dbs == nil {
		s.dbs = map[string]*sql.DB{}
	}
	s.dbs[database], s.driver = db, driver
	return db, driver, nil
}

// Open is the connection to the URL's database, or to the database of tenant (an fqdn) found in the central one.
func (s *Source) Open(ctx context.Context, tenant string) (*sql.DB, string, error) {
	db, driver, err := s.conn("")
	if err != nil || tenant == "" {
		return db, driver, err
	}
	name, err := tenantDatabase(ctx, db, driver, s.TenantQuery, tenant)
	if err != nil {
		return nil, "", err
	}
	return s.conn(name)
}

// TenantNames are up to 4 host names of the tenants in the central database (Hyn Tenancy), or none when it cannot tell: with a TenantQuery of its own the
// layout is not known.
func (s *Source) TenantNames(ctx context.Context) []string {
	if s.TenantQuery != "" && s.TenantQuery != HynTenantQuery {
		return nil
	}
	db, _, err := s.conn("")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, QueryTimeout)
	defer cancel()
	rows, err := db.QueryContext(ctx, "SELECT fqdn FROM hostnames WHERE deleted_at IS NULL ORDER BY id LIMIT 4")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			names = append(names, n)
		}
	}
	return names
}

// OnlyTenant is the host name of the tenant when the central database (Hyn Tenancy) has exactly one, or "" when it has none, several, or cannot tell.
func (s *Source) OnlyTenant(ctx context.Context) string {
	if s.TenantQuery != "" && s.TenantQuery != HynTenantQuery {
		return ""
	}
	db, _, err := s.conn("")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, QueryTimeout)
	defer cancel()
	rows, err := db.QueryContext(ctx, "SELECT fqdn FROM hostnames WHERE deleted_at IS NULL ORDER BY id LIMIT 2")
	if err != nil {
		return ""
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			names = append(names, n)
		}
	}
	if len(names) != 1 {
		return ""
	}
	return names[0]
}

// TenantHint is what to add to the error of a call with no tenant when a table is missing: the tables of the app are in the tenant's database, with the host
// names of the central one. It is "" with a tenant and for any other error.
func (s *Source) TenantHint(ctx context.Context, msg, tenant string) string {
	if tenant != "" || !(strings.Contains(msg, "Error 1146") || strings.Contains(msg, "no such table")) {
		return ""
	}
	hint := "; the call named no tenant, so it read the central database: pass tenant to read the one of the app"
	if names := s.TenantNames(ctx); len(names) > 0 {
		hint += " (host names: " + strings.Join(names, ", ") + ")"
	}
	return hint
}

// Answer checks the statement, runs it and returns the text for the agent; no error text carries the password of the URL.
func (s *Source) Answer(ctx context.Context, in QueryArgs) (string, error) {
	pw := Password(s.URL)
	fail := func(err error) (string, error) { return "", errors.New(Redact(err.Error(), pw)) }
	stmt, err := CheckSQL(in.SQL)
	if err != nil {
		return fail(err)
	}
	db, driver, err := s.Open(ctx, in.Tenant)
	if err != nil {
		return fail(err)
	}
	out, err := runQuery(ctx, db, driver, stmt, rowLimit(in.Limit))
	if err != nil {
		return "", s.Annotate(ctx, db, driver, stmt, in.Tenant, err)
	}
	return out, nil
}

// QueryError is the failure of a statement that reached the database; Tenant is the one the call named.
type QueryError struct{ Msg, Tenant string }

func (e *QueryError) Error() string { return e.Msg }

// Annotate turns the error of a statement run on conn into a QueryError: the text gains the real columns or the tenant hint, and loses the URL password.
func (s *Source) Annotate(ctx context.Context, conn *sql.DB, driver, stmt, tenant string, err error) error {
	msg := err.Error() + tableHint(ctx, conn, driver, err.Error()) + columnHint(ctx, conn, driver, stmt, err.Error()) + s.TenantHint(ctx, err.Error(), tenant)
	return &QueryError{Msg: Redact(msg, Password(s.URL)), Tenant: tenant}
}

var (
	fromTable   = regexp.MustCompile("(?i)\\b(?:from|join)\\s+((?:[`\"]?[\\p{L}\\p{N}_$]+[`\"]?\\.)?[`\"]?[\\p{L}\\p{N}_$]+[`\"]?)")
	badColumn   = regexp.MustCompile(`Unknown column '([^']+)'|no such column: (\S+)`)
	maxHintCols = 40 // column names shown in all, spread over the tables
	maxHintTabs = 5
)

// columnHint is the text appended to an unknown-column or unknown-table error: the real columns of the tables in the statement's FROM and JOIN
// that exist, those that resemble the missing column first. "" when the error is another one or no table is found; commas in FROM are not read.
func columnHint(ctx context.Context, db *sql.DB, driver, stmt, msg string) string {
	if !strings.Contains(msg, "Error 1054") && !strings.Contains(msg, "no such column") && !strings.Contains(msg, "Error 1146") && !strings.Contains(msg, "no such table") {
		return ""
	}
	want := ""
	if m := badColumn.FindStringSubmatch(msg); m != nil {
		want = strings.ToLower(m[1] + m[2])
		want = want[strings.LastIndex(want, ".")+1:]
	}
	ctx, cancel := context.WithTimeout(ctx, QueryTimeout)
	defer cancel()
	q := "SELECT name FROM pragma_table_info(?)"
	if driver == "mysql" {
		q = "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION"
	}
	var names, parts, every []string
	for _, m := range fromTable.FindAllStringSubmatch(stmt, -1) {
		if name := strings.Trim(m[1][strings.LastIndex(m[1], ".")+1:], "`\""); !slices.Contains(names, name) && len(names) < maxHintTabs {
			names = append(names, name)
		}
	}
	for _, name := range names {
		rows, err := db.QueryContext(ctx, q, name)
		if err != nil {
			continue
		}
		var near, rest []string
		for rows.Next() {
			var c string
			if rows.Scan(&c) != nil {
				break
			}
			if l := strings.ToLower(c); want != "" && (strings.Contains(l, want) || strings.Contains(want, l)) {
				near = append(near, c)
			} else {
				rest = append(rest, c)
			}
		}
		rows.Close()
		every = append(every, near...)
		every = append(every, rest...)
		cols := append(near, rest...)
		if len(cols) == 0 {
			continue
		}
		if room := max(10, maxHintCols/len(names)); len(cols) > room {
			cols = append(cols[:room], "…")
		}
		parts = append(parts, name+"("+strings.Join(cols, ", ")+")")
	}
	if len(parts) == 0 {
		return ""
	}
	return closest(want, every) + ". Columns of the tables in FROM/JOIN: " + strings.Join(parts, "; ")
}

var badTable = regexp.MustCompile(`Table '(?:[^'.]*\.)?([^']+)' doesn't exist|no such table: (\S+)`)

const maxHintPrefixTables = 15

// tableHint is the text appended to an unknown-table error: the nearest existing table and the tables that share the missing one's prefix up to its first
// underscore. "" when the error is another one or nothing is close.
func tableHint(ctx context.Context, db *sql.DB, driver, msg string) string {
	m := badTable.FindStringSubmatch(msg)
	if m == nil {
		return ""
	}
	want := strings.ToLower(m[1] + m[2])
	ctx, cancel := context.WithTimeout(ctx, QueryTimeout)
	defer cancel()
	q := "SELECT name FROM sqlite_master WHERE type IN ('table','view') ORDER BY name"
	if driver == "mysql" {
		q = "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME"
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var every, same []string
	prefix, _, ok := strings.Cut(want, "_")
	for rows.Next() {
		var n string
		if rows.Scan(&n) != nil {
			break
		}
		every = append(every, n)
		if ok && strings.HasPrefix(strings.ToLower(n), prefix+"_") && len(same) < maxHintPrefixTables {
			same = append(same, n)
		}
	}
	out := closest(want, every)
	if len(same) > 0 {
		out += ". Tables with prefix «" + prefix + "_»: " + strings.Join(same, ", ")
	}
	return out
}

// closest is ". Did you mean «col»?" for the column nearest to want (a few edits away, or the only one that contains it or is contained in it), "" when none is close.
func closest(want string, cols []string) string {
	if want == "" {
		return ""
	}
	best, bestD, within := "", 1<<30, []string{}
	for _, c := range cols {
		l := strings.ToLower(c)
		if strings.Contains(l, want) || strings.Contains(want, l) {
			within = append(within, c)
		}
		if d := editDistance(want, l); d < bestD {
			best, bestD = c, d
		}
	}
	if bestD <= max(2, len(want)/4) && bestD < len([]rune(want)) {
		return ". Did you mean «" + best + "»?"
	}
	if len(within) == 1 {
		return ". Did you mean «" + within[0] + "»?"
	}
	return ""
}

// editDistance is the Levenshtein distance between a and b, in letters.
func editDistance(a, b string) int {
	x, y := []rune(a), []rune(b)
	row := make([]int, len(y)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(x); i++ {
		prev := row[0]
		row[0] = i
		for j := 1; j <= len(y); j++ {
			cur := row[j]
			cost := 1
			if x[i-1] == y[j-1] {
				cost = 0
			}
			row[j] = min(row[j]+1, row[j-1]+1, prev+cost)
			prev = cur
		}
	}
	return row[len(y)]
}

// openDB opens mysql://user:password@host:port/db or sqlite:///path/to/file.db. The SQLite file is opened with query_only on. A non-empty
// database replaces the one in the URL: the MySQL database name, or the SQLite file <database>.db in the same folder.
func openDB(raw, database string) (*sql.DB, string, error) {
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
		if database != "" {
			cfg.DBName = database
		}
		cfg.Timeout, cfg.ReadTimeout = 5*time.Second, QueryTimeout
		c, err := mysql.NewConnector(cfg)
		if err != nil {
			return nil, "", err
		}
		return sql.OpenDB(c), "mysql", nil
	case "sqlite":
		path := u.Host + u.Path
		if database != "" {
			path = filepath.Join(filepath.Dir(path), database+".db")
		}
		db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=query_only(1)") // mode=ro: the file is never written or created
		return db, "sqlite", err
	}
	return nil, "", fmt.Errorf("unsupported database URL scheme %q: use mysql or sqlite", u.Scheme)
}

// Password is the password inside a database URL, or "".
func Password(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.User != nil {
		pw, _ := u.User.Password()
		return pw
	}
	return ""
}

// Redact hides every occurrence of secret in msg.
func Redact(msg, secret string) string {
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

// runQuery runs one checked statement in a read-only transaction that is always rolled back (the second layer after CheckSQL: on MySQL the
// transaction is READ ONLY, on SQLite the file is opened with query_only) and returns a TSV table with a header and a closing line that says
// whether rows were left out.
func runQuery(ctx context.Context, db *sql.DB, driver, stmt string, limit int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeout)
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
	header := strings.Join(cols, "\t") // every column name stays: the values are positional; only an absurd header is cut
	if len(header) > input.MaxAnswer/2 {
		n := input.MaxAnswer / 2
		for n > 0 && !utf8.RuneStart(header[n]) {
			n--
		}
		header = header[:n] + "…"
	}
	b.WriteString(header + "\n")
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
		if b.Len()+len(row) > input.MaxAnswer {
			if n == 0 { // the first row alone is too big: show its start rather than nothing, and never more than input.MaxAnswer
				room := max(0, input.MaxAnswer-b.Len())
				for room > 0 && !utf8.RuneStart(row[room]) {
					room--
				}
				b.WriteString(row[:room] + "\n")
				n++
			}
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
		fmt.Fprintf(&b, "(%d rows shown, cut at %d bytes: select fewer columns or add WHERE)", n, input.MaxAnswer)
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
	if len(s) > 4*maxCell { // cut before the conversion: a 60 MB value must not cost 240 MB of runes
		n := 4 * maxCell
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
		s = s[:n]
	}
	s = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
	if r := []rune(s); len(r) > maxCell {
		s = string(r[:maxCell]) + "…"
	}
	return s
}

var writeVerb = regexp.MustCompile(`(?i)must start with .* not "(insert|update|delete|replace|create|alter|drop|truncate|merge|call|grant|revoke)"`)

// Failure is the text of a failed call: what happened and what to do next. tenant is the one the call named, if any.
func Failure(err error, tenant string) string {
	next := "fix the statement or the connection (WEBSCOPE_DB_URL) and call again."
	switch msg := err.Error(); {
	case writeVerb.MatchString(msg) || strings.Contains(msg, " is a write"):
		next = "webscope never writes to the database: make the change through the screen (test rows start with ZZ_) and read the result with query."
	case strings.Contains(msg, "Error 1054") || strings.Contains(msg, "no such column"):
		next = "that column does not exist: code(fields {table}) or SHOW COLUMNS FROM table lists the real ones; then call again."
	case strings.Contains(msg, "Error 1146") || strings.Contains(msg, "no such table"):
		next = "that table does not exist: SHOW TABLES LIKE '%word%' lists the tables; then call again."
		if tenant != "" {
			next = "that table does not exist in the database of tenant " + tenant + ": SHOW TABLES LIKE '%word%' lists its tables; a central table is read by calling without tenant."
		}
	}
	return "Failed: " + strings.TrimRight(err.Error(), ". ") + ". Next: " + next
}
