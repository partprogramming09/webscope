package code

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/partprogramming09/webscope/internal/db"
)

// TestACallWithNoTenantUsesTheOnlyTenantOfTheCentralDatabase: when a table is missing from the central database and it holds exactly one tenant, rows reads
// that tenant and says so; with two tenants the call still fails and lists them.
func TestACallWithNoTenantUsesTheOnlyTenantOfTheCentralDatabase(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, stmts ...string) {
		conn, err := sql.Open("sqlite", filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		for _, q := range stmts {
			if _, err := conn.Exec(q); err != nil {
				t.Fatalf("%s: %q: %v", name, q, err)
			}
		}
	}
	write("central.db",
		"CREATE TABLE websites (id INTEGER PRIMARY KEY, uuid TEXT, managed_by_database_connection TEXT, deleted_at TEXT)",
		"CREATE TABLE hostnames (id INTEGER PRIMARY KEY, fqdn TEXT, website_id INTEGER, deleted_at TEXT)",
		"INSERT INTO websites VALUES (1, 'aaa111', NULL, NULL), (2, 'bbb222', NULL, NULL)",
		"INSERT INTO hostnames VALUES (1, 'one.test', 1, NULL)")
	write("aaa111.db", "CREATE TABLE tenant_only (id INTEGER, name TEXT)", "INSERT INTO tenant_only VALUES (1, 'a'), (2, 'b')")
	c := codeSetup(t)
	c.Src = &db.Source{URL: "sqlite://" + filepath.Join(dir, "central.db"), TenantQuery: db.HynTenantQuery}
	args := map[string]any{"table": "tenant_only", "a": float64(1), "b": float64(2)}
	out, err := code(t, c, "rows", args)
	if err != nil || !strings.Contains(out, "tenant one.test: the only one in the central database") || !strings.Contains(out, "name") {
		t.Errorf("one tenant: the call reads it and says so: %v\n%s", err, out)
	}
	write("central.db", "INSERT INTO hostnames VALUES (2, 'two.test', 2, NULL)")
	c.Src = &db.Source{URL: "sqlite://" + filepath.Join(dir, "central.db"), TenantQuery: db.HynTenantQuery}
	if _, err := code(t, c, "rows", args); err == nil || !strings.Contains(err.Error(), "no such table") {
		t.Errorf("two tenants: the call is not guessed and fails: %v", err)
	}
	write("central.db", "DELETE FROM hostnames WHERE id = 2")
	c.Src = &db.Source{URL: "sqlite://" + filepath.Join(dir, "central.db"), TenantQuery: "SELECT uuid FROM websites WHERE id = 1 AND ? <> ''"}
	if _, err := code(t, c, "rows", args); err == nil || !strings.Contains(err.Error(), "no such table") {
		t.Errorf("a tenant query of the user's own: the layout is not known, nothing is guessed: %v", err)
	}
	c.Src = &db.Source{URL: "sqlite://" + filepath.Join(dir, "central.db"), TenantQuery: db.HynTenantQuery}
	args["tenant"] = "one.test"
	if out, err := code(t, c, "rows", args); err != nil || strings.Contains(out, "the only one") {
		t.Errorf("a tenant that is named is used as it is: %v\n%s", err, out)
	}
}
