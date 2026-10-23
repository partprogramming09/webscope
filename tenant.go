package main

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
)

// validName is what a tenant's database name may be: it ends up in a connection, so it must not carry anything else.
var validName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// tenantDatabase asks the central database for the database of tenant. lookup has one ? (the tenant) and returns the database name in its
// first column; if a second column has a value the tenant is managed some other way, which this version does not support.
// No row, more than one row and an invalid name are errors: nothing is chosen for the caller.
func tenantDatabase(ctx context.Context, db *sql.DB, driver, lookup, tenant string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: driver == "mysql"})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, lookup, tenant)
	if err != nil {
		return "", fmt.Errorf("tenant lookup failed: %w", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	var names []string
	var managed sql.NullString
	for rows.Next() && len(names) < 2 {
		cells := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", err
		}
		names = append(names, cells[0].String)
		if len(cols) > 1 && cells[1].Valid && cells[1].String != "" {
			managed = cells[1]
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	switch {
	case len(names) == 0:
		return "", fmt.Errorf("tenant %q not found in the central database", tenant)
	case len(names) > 1:
		return "", fmt.Errorf("tenant %q is ambiguous: more than one database matches", tenant)
	case managed.Valid:
		return "", fmt.Errorf("tenant %q uses managed_by_database_connection (%s): not supported", tenant, managed.String)
	case !validName.MatchString(names[0]):
		return "", fmt.Errorf("the database name of tenant %q is not valid", tenant)
	}
	return names[0], nil
}
