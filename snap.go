package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	maxSnapRows  = 2000 // rows kept per snapshot
	maxSnapshots = 20
)

var snapName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

// snapshot is a query and the rows it returned, by key. NULL is kept as \N (shortcut: a text value that is exactly \N is read as NULL).
type snapshot struct {
	sql, key, tenant string
	cols             []string
	rows             map[string][]string
}

// rawCell is a value as text, whole: \N for NULL.
func rawCell(v any) string {
	switch x := v.(type) {
	case nil:
		return `\N`
	case []byte:
		return string(x)
	case time.Time:
		return x.Format("2006-01-02 15:04:05")
	}
	return fmt.Sprint(v)
}

// readAll runs a checked read-only statement and returns its columns and rows; more than maxSnapRows is an error, not a partial snapshot.
func (c *coder) readAll(ctx context.Context, stmt, tenant string) ([]string, [][]string, error) {
	db, driver, err := c.s.open(ctx, tenant)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: driver == "mysql"})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, stmt)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	var out [][]string
	vals, ptrs := make([]any, len(cols)), make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if len(out) == maxSnapRows {
			return nil, nil, fmt.Errorf("more than %d rows: add a WHERE to keep fewer", maxSnapRows)
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		r := make([]string, len(cols))
		for i, v := range vals {
			r[i] = rawCell(v)
		}
		out = append(out, r)
	}
	return cols, out, rows.Err()
}

// keyed indexes rows by the key column.
func keyed(cols []string, rows [][]string, key string) (map[string][]string, error) {
	k := slices.Index(cols, key)
	if k < 0 {
		return nil, fmt.Errorf("the key %q is not a column of the query (%s)", key, strings.Join(cols, ", "))
	}
	out := make(map[string][]string, len(rows))
	for _, r := range rows {
		if _, dup := out[r[k]]; dup {
			return nil, fmt.Errorf("the key column %s is not unique (the value %q repeats): choose another with key", key, show(r[k]))
		}
		out[r[k]] = r
	}
	return out, nil
}

func (c *coder) snapSave(ctx context.Context, args map[string]any) (string, error) {
	name := str(args, "name")
	if !snapName.MatchString(name) {
		return "", fmt.Errorf("the snapshot name %q is not valid: letters, digits, _ and - up to 40", name)
	}
	stmt, err := checkSQL(str(args, "sql"))
	if err != nil {
		return "", err
	}
	tenant := str(args, "tenant")
	cols, rows, err := c.readAll(ctx, stmt, tenant)
	if err != nil {
		return "", err
	}
	if len(cols) == 0 {
		return "", errors.New("the query returns no columns")
	}
	key := str(args, "key")
	if key == "" {
		key = cols[0]
	}
	by, err := keyed(cols, rows, key)
	if err != nil {
		return "", err
	}
	c.snapMu.Lock()
	defer c.snapMu.Unlock()
	if _, ok := c.snaps[name]; !ok && len(c.snaps) >= maxSnapshots {
		return "", fmt.Errorf("%d snapshots are kept already: reuse a name", maxSnapshots)
	}
	if c.snaps == nil {
		c.snaps = map[string]*snapshot{}
	}
	c.snaps[name] = &snapshot{stmt, key, tenant, cols, by}
	return fmt.Sprintf("snap «%s»: %d rows saved, key %s, kept in memory. Next: do the action on the screen, then snap_diff {name}.", name, len(rows), key), nil
}

// val is a value for the answer: ” for an empty text, one line, 40 characters at most.
func val(s string) string {
	if s == "" {
		return "''"
	}
	return show(strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s))
}

func (c *coder) snapDiff(ctx context.Context, args map[string]any) (string, error) {
	name := str(args, "name")
	c.snapMu.Lock()
	s, ok := c.snaps[name]
	var known []string
	for n := range c.snaps {
		known = append(known, n)
	}
	c.snapMu.Unlock()
	if !ok {
		sort.Strings(known)
		return "", fmt.Errorf("no snapshot named %q (kept: %s)", name, strings.Join(known, ", "))
	}
	cols, rows, err := c.readAll(ctx, s.sql, s.tenant)
	if err != nil {
		return "", err
	}
	if !slices.Equal(cols, s.cols) {
		return "", errors.New("the columns of the query changed since the snapshot")
	}
	now, err := keyed(cols, rows, s.key)
	if err != nil {
		return "", err
	}
	var changed, gone, added []string
	for _, k := range slices.Sorted(mapKeys(s.rows)) {
		a, still := now[k]
		if !still {
			gone = append(gone, "deleted "+val(k))
			continue
		}
		var d []string
		for i, col := range cols {
			if s.rows[k][i] != a[i] {
				d = append(d, fmt.Sprintf("%s %s → %s", col, val(s.rows[k][i]), val(a[i])))
			}
		}
		if len(d) > 0 {
			changed = append(changed, fmt.Sprintf("changed %s: %s", val(k), strings.Join(d, ", ")))
		}
	}
	for _, k := range slices.Sorted(mapKeys(now)) {
		if _, was := s.rows[k]; was {
			continue
		}
		var d []string
		for i, col := range cols {
			if col != s.key && len(d) < 6 {
				d = append(d, col+"="+val(now[k][i]))
			}
		}
		added = append(added, fmt.Sprintf("new %s: %s", val(k), strings.Join(d, ", ")))
	}
	head := fmt.Sprintf("snap «%s»: %d rows before, %d now", name, len(s.rows), len(now))
	all := append(append(changed, gone...), added...)
	if len(all) == 0 {
		return head + ": no changes", nil
	}
	return capOut(head + "\n" + strings.Join(all, "\n")), nil
}

func mapKeys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}
