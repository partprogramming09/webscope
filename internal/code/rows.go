package code

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/partprogramming09/webscope/internal/db"
	"github.com/partprogramming09/webscope/internal/input"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// whereOf is {column: value} of an argument as sorted columns and their values, every name checked as a plain identifier.
func whereOf(args map[string]any, name string) (cols []string, vals []any, err error) {
	m, _ := args[name].(map[string]any)
	if len(m) == 0 {
		switch v := args[name].(type) { // an id: the row whose key column (id by default) has it
		case string, float64:
			key := input.Str(args, "key")
			if key == "" {
				key = "id"
			}
			if !identifier.MatchString(key) {
				return nil, nil, fmt.Errorf("key: %q is not a column name (letters, digits and _)", echo(key))
			}
			return []string{key}, []any{fmt.Sprint(v)}, nil
		}
		return nil, nil, fmt.Errorf("rows needs %s: {column: value} that picks one row, or an id (the column is key, id by default)", name)
	}
	for k := range m {
		if !identifier.MatchString(k) {
			return nil, nil, fmt.Errorf("%s: %q is not a column name (letters, digits and _)", name, echo(k))
		}
		cols = append(cols, k)
	}
	sort.Strings(cols)
	for _, k := range cols {
		vals = append(vals, fmt.Sprint(m[k]))
	}
	return cols, vals, nil
}

// readTx runs one parameterised read in a read-only transaction and hands the rows to scan.
func (c *Tool) readTx(ctx context.Context, tenant, stmt string, params []any, scan func(*sql.Rows) error) error {
	conn, driver, err := c.Src.Open(ctx, tenant)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, db.QueryTimeout)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: driver == "mysql"})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, stmt, params...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scan(rows)
}

// oneRow is the single row of table that the where columns pick: all its columns, NULL as nil.
func (c *Tool) oneRow(ctx context.Context, tenant, table, side string, cols []string, vals []any) ([]string, []*string, error) {
	conds := make([]string, len(cols))
	for i, k := range cols {
		conds[i] = "`" + k + "` = ?"
	}
	var names []string
	var found [][]*string
	err := c.readTx(ctx, tenant, "SELECT * FROM `"+table+"` WHERE "+strings.Join(conds, " AND ")+" LIMIT 2", vals, func(rows *sql.Rows) (err error) {
		if names, err = rows.Columns(); err != nil {
			return err
		}
		for rows.Next() && len(found) < 2 {
			cells := make([]sql.NullString, len(names))
			ptrs := make([]any, len(names))
			for i := range cells {
				ptrs[i] = &cells[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return err
			}
			r := make([]*string, len(names))
			for i := range cells {
				if cells[i].Valid {
					r[i] = &cells[i].String
				}
			}
			found = append(found, r)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, nil, err
	}
	switch len(found) {
	case 0:
		return nil, nil, fmt.Errorf("no row in %s matches %s", echo(table), side)
	case 1:
		return names, found[0], nil
	}
	return nil, nil, fmt.Errorf("more than one row in %s matches %s: add a column to it", echo(table), side)
}

// countRows is how many rows of table have col equal to val.
func (c *Tool) countRows(ctx context.Context, tenant, table, col string, val any) (n int, err error) {
	err = c.readTx(ctx, tenant, "SELECT COUNT(*) FROM `"+table+"` WHERE `"+col+"` = ?", []any{val}, func(rows *sql.Rows) error {
		if rows.Next() {
			return rows.Scan(&n)
		}
		return rows.Err()
	})
	return n, err
}

// cellClass says how two values of one column differ without printing either; "" when they are equal.
func cellClass(a, b *string) string {
	kind := func(s *string) string {
		switch {
		case s == nil:
			return "NULL"
		case *s == "":
			return "empty"
		}
		return "filled"
	}
	if a == nil || b == nil || *a == "" || *b == "" {
		if ka, kb := kind(a), kind(b); ka != kb {
			return fmt.Sprintf("a %s, b %s", ka, kb)
		}
		return ""
	}
	switch fa, fb := strings.TrimSpace(*a), strings.TrimSpace(*b); {
	case *a == *b:
		return ""
	case strings.EqualFold(*a, *b):
		return "case only"
	case fa == fb:
		return "spaces only"
	case sameNumber(fa, fb):
		return "same number, other text"
	}
	return fmt.Sprintf("differs (a %d chars, b %d chars)", utf8.RuneCountInString(*a), utf8.RuneCountInString(*b))
}

// sameNumber is true when both texts are numbers of the same value, such as 7 and 007.
func sameNumber(a, b string) bool {
	x, errA := strconv.ParseFloat(a, 64)
	y, errB := strconv.ParseFloat(b, 64)
	return errA == nil && errB == nil && x == y
}

// rows compares two rows of one table, usually the one each of two systems wrote for the same action, column by column and without printing a value;
// children counts the rows of other tables that point at each one.
func (c *Tool) rows(ctx context.Context, args map[string]any) (string, error) {
	table, tenant := input.Str(args, "table"), input.Str(args, "tenant")
	if !identifier.MatchString(table) {
		return "", errors.New("rows needs table (letters, digits and _), a, b ({column: value} that picks one row each) and, optionally, children ({table: column})")
	}
	aCols, aVals, err := whereOf(args, "a")
	if err != nil {
		return "", err
	}
	bCols, bVals, err := whereOf(args, "b")
	if err != nil {
		return "", err
	}
	children, _ := args["children"].(map[string]any)
	for t, col := range children {
		if !identifier.MatchString(t) || !identifier.MatchString(fmt.Sprint(col)) {
			return "", fmt.Errorf("children: %q and its column must be plain names (letters, digits and _)", echo(t))
		}
	}
	names, ra, err := c.oneRow(ctx, tenant, table, "a", aCols, aVals)
	if err != nil {
		return "", err
	}
	_, rb, err := c.oneRow(ctx, tenant, table, "b", bCols, bVals)
	if err != nil {
		return "", err
	}
	label := func(cols []string, vals []any) string {
		var l []string
		for i, k := range cols {
			l = append(l, fmt.Sprintf("%s=%v", k, echo(fmt.Sprint(vals[i]))))
		}
		return strings.Join(l, ", ")
	}
	var diffs []string
	compared, equal := 0, 0
	for i, name := range names {
		isKey := func(k string) bool { return strings.EqualFold(k, name) }
		if slices.ContainsFunc(aCols, isKey) || slices.ContainsFunc(bCols, isKey) {
			continue
		}
		compared++
		if cl := cellClass(ra[i], rb[i]); cl != "" {
			diffs = append(diffs, name+": "+cl)
		} else {
			equal++
		}
	}
	out := []string{fmt.Sprintf("rows %s: a (%s) vs b (%s): %d columns, %d equal, %d differ. The columns that pick the rows are not compared; no value is printed.", echo(table), label(aCols, aVals), label(bCols, bVals), compared, equal, len(diffs))}
	if len(diffs) == 0 {
		out = append(out, "no differences")
	}
	out = append(out, diffs...)
	if len(children) > 0 {
		out = append(out, "## rows that point at each (by the first column of a and of b)")
		var tables []string
		for t := range children {
			tables = append(tables, t)
		}
		sort.Strings(tables)
		for _, t := range tables {
			col := fmt.Sprint(children[t])
			ka, names, err := c.childValues(ctx, tenant, t, col, aVals[0])
			if err != nil {
				return "", fmt.Errorf("children %s: %w", echo(t), err)
			}
			kb, _, err := c.childValues(ctx, tenant, t, col, bVals[0])
			if err != nil {
				return "", fmt.Errorf("children %s: %w", echo(t), err)
			}
			l := fmt.Sprintf("%s: a %d, b %d", t, ka.n, kb.n)
			if ka.n != kb.n {
				l += " ≠"
			} else if ka.n > 0 && ka.n <= maxChildRows {
				var differ []string
				for i, name := range names {
					if i > 0 && name != col && !slices.Equal(ka.cols[i], kb.cols[i]) { // the first column is the id of each child row: it always differs
						differ = append(differ, name)
					}
				}
				if len(differ) > 0 {
					l += "; values differ in: " + strings.Join(differ, ", ")
				} else {
					l += "; the same values in every column"
				}
			}
			out = append(out, l)
		}
	}
	return capOut(strings.Join(out, "\n")), nil
}

// maxChildRows is how many child rows of each side rows reads to compare their columns.
const maxChildRows = 200

// childSet is the number of rows of a child table that point at one row and, if there are few, the values of each column, sorted (never shown).
type childSet struct {
	n    int
	cols [][]string
}

// childValues reads the rows of table whose col is val: how many, and the sorted values of each column when there are at most maxChildRows. A column that
// does not exist is refused with the columns the table has.
func (c *Tool) childValues(ctx context.Context, tenant, table, col string, val any) (childSet, []string, error) {
	var set childSet
	var names []string
	err := c.readTx(ctx, tenant, "SELECT * FROM `"+table+"` WHERE `"+col+"` = ? LIMIT "+strconv.Itoa(maxChildRows+1), []any{val}, func(rows *sql.Rows) error {
		var err error
		if names, err = rows.Columns(); err != nil {
			return err
		}
		set.cols = make([][]string, len(names))
		for rows.Next() {
			cells := make([]sql.NullString, len(names))
			ptrs := make([]any, len(names))
			for i := range cells {
				ptrs[i] = &cells[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return err
			}
			set.n++
			for i, cell := range cells {
				v := "\x00NULL"
				if cell.Valid {
					v = cell.String
				}
				set.cols[i] = append(set.cols[i], v)
			}
		}
		return rows.Err()
	})
	if err != nil {
		if cols, e2 := c.columnNames(ctx, tenant, table); e2 == nil && !slices.Contains(cols, col) {
			return set, nil, fmt.Errorf("%q is not a column; columns of %s: %s", echo(col), table, strings.Join(cols, ", "))
		}
		return set, nil, err
	}
	if set.n > maxChildRows {
		set.n = maxChildRows + 1
	}
	for i := range set.cols {
		slices.Sort(set.cols[i])
	}
	return set, names, nil
}

// columnNames are the columns of a table, read with a SELECT that returns no row.
func (c *Tool) columnNames(ctx context.Context, tenant, table string) (cols []string, err error) {
	err = c.readTx(ctx, tenant, "SELECT * FROM `"+table+"` LIMIT 0", nil, func(rows *sql.Rows) (e error) {
		cols, e = rows.Columns()
		return e
	})
	return cols, err
}

var leftoverPrefix = regexp.MustCompile(`^[A-Za-z0-9_-]{1,20}$`)

// leftoverColumns are the columns to count: column, then columns (a list), at most maxLeftoverColumns.
func leftoverColumns(args map[string]any) ([]string, error) {
	var cols []string
	if c := input.Str(args, "column"); c != "" {
		cols = append(cols, c)
	}
	if l, ok := args["columns"].([]any); ok {
		for _, v := range l {
			s, _ := v.(string)
			cols = append(cols, s)
		}
	}
	if len(cols) == 0 || len(cols) > maxLeftoverColumns {
		return nil, fmt.Errorf("leftovers needs table and column or columns (1 to %d): letters, digits and _ only", maxLeftoverColumns)
	}
	return cols, nil
}

const maxLeftoverColumns = 10

// leftovers counts the rows of table whose column (or each of columns) starts with prefix (ZZ_ by default), the mark of the rows a test wrote: a count, never a value.
func (c *Tool) leftovers(ctx context.Context, args map[string]any) (string, error) {
	table, prefix := input.Str(args, "table"), input.Str(args, "prefix")
	if prefix == "" {
		prefix = "ZZ_"
	}
	cols, err := leftoverColumns(args)
	if err != nil {
		return "", err
	}
	if !db.ValidName.MatchString(table) {
		return "", errors.New("leftovers needs table and column: letters, digits and _ only")
	}
	for _, col := range cols {
		if !db.ValidName.MatchString(col) {
			return "", errors.New("leftovers needs table and column: letters, digits and _ only")
		}
	}
	if !leftoverPrefix.MatchString(prefix) {
		return "", fmt.Errorf("the prefix must be 1 to 20 letters, digits, _ or -")
	}
	like := strings.NewReplacer("_", "!_", "%", "!%").Replace(prefix) + "%"
	markCol, markVal, err := deletedMark(args)
	if err != nil {
		return "", err
	}
	count := func(where string) (string, error) {
		stmt, err := db.CheckSQL(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, where))
		if err != nil {
			return "", err
		}
		_, rows, err := c.readAll(ctx, stmt, input.Str(args, "tenant"))
		if err != nil {
			return "", err
		}
		if len(rows) != 1 || len(rows[0]) != 1 {
			return "", errors.New("the count did not come back")
		}
		return rows[0][0], nil
	}
	var lines []string
	live := false
	for _, col := range cols {
		match := fmt.Sprintf("%s LIKE '%s' ESCAPE '!'", col, like)
		n, err := count(match)
		if err != nil {
			return "", err
		}
		line := fmt.Sprintf("leftovers %s.%s starting with %s: %s rows", table, col, prefix, n)
		if markCol == "" {
			live = live || n != "0"
		} else {
			m, err := count(fmt.Sprintf("%s AND %s = '%s'", match, markCol, markVal))
			if err != nil {
				return "", err
			}
			nn, mm := atoiOr0(n), atoiOr0(m)
			line += fmt.Sprintf(" (%s marked %s=%s, %d live)", m, markCol, markVal, nn-mm)
			live = live || nn-mm > 0
		}
		lines = append(lines, line)
	}
	out := strings.Join(lines, "\n")
	switch {
	case live:
		out += ". They are test rows: clean them through the screen, not through the database, and say in the report that they were left."
	case markCol != "":
		out += ". Every one of them is marked " + markCol + "=" + markVal + ": nothing is left to clean."
	}
	return out, nil
}

var deletedValue = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

// deletedMark is the one {column: value} of deleted, the mark of a soft delete, or two empty texts when there is none.
func deletedMark(args map[string]any) (col, val string, err error) {
	raw, ok := args["deleted"]
	if !ok {
		return "", "", nil
	}
	m, ok := raw.(map[string]any)
	if !ok || len(m) != 1 {
		return "", "", errors.New("deleted is {column: value}, one entry: the mark of a soft delete, such as {estado: BORRADO}")
	}
	for col, v := range m {
		val = fmt.Sprint(v)
		if !db.ValidName.MatchString(col) || !deletedValue.MatchString(val) {
			return "", "", errors.New("deleted: the column must be a plain name and the value 1 to 40 letters, digits, _ or -")
		}
		return col, val, nil
	}
	return "", "", nil
}

// atoiOr0 is the number in s, or 0.
func atoiOr0(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
