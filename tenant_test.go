package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tenants writes a central SQLite file with the two tables of Hyn Tenancy and one file per tenant database, each with its own items.
func tenants(t *testing.T) (*source, string) {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, stmts ...string) {
		db, err := sql.Open("sqlite", filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		for _, q := range stmts {
			if _, err := db.Exec(q); err != nil {
				t.Fatalf("%s: %q: %v", name, q, err)
			}
		}
	}
	write("central.db",
		"CREATE TABLE items (id INTEGER, name TEXT)", "INSERT INTO items VALUES (1, 'from-central')",
		"CREATE TABLE websites (id INTEGER PRIMARY KEY, uuid TEXT, managed_by_database_connection TEXT, deleted_at TEXT)",
		"CREATE TABLE hostnames (id INTEGER PRIMARY KEY, fqdn TEXT, website_id INTEGER, deleted_at TEXT)",
		"INSERT INTO websites VALUES (1, 'aaa111', NULL, NULL), (2, 'bbb222', NULL, NULL), (3, 'ccc333', 'other', NULL), (4, 'ddd444', NULL, '2026-01-01'), (5, 'bad;name', NULL, NULL)",
		"INSERT INTO hostnames VALUES (1, 'one.test', 1, NULL), (2, 'two.test', 2, NULL), (3, 'managed.test', 3, NULL), (4, 'gone.test', 4, NULL), "+
			"(5, 'dup.test', 1, NULL), (6, 'dup.test', 2, NULL), (7, 'evil.test', 5, NULL), (8, 'removed-host.test', 1, '2026-01-01')")
	for uuid, who := range map[string]string{"aaa111": "from-aaa", "bbb222": "from-bbb"} {
		write(uuid+".db", "CREATE TABLE items (id INTEGER, name TEXT)", "INSERT INTO items VALUES (1, '"+who+"')", "INSERT INTO items VALUES (2, 'second')")
	}
	return &source{url: "sqlite://" + filepath.Join(dir, "central.db"), tenantQuery: hynTenantQuery}, dir
}

func askTenant(t *testing.T, s *source, tenant, q string) (string, error) {
	t.Helper()
	return s.answer(context.Background(), queryArgs{SQL: q, Tenant: tenant})
}

func TestATenantQueryReadsThatTenantsDatabase(t *testing.T) {
	s, _ := tenants(t)
	for tenant, want := range map[string]string{"": "from-central", "one.test": "from-aaa", "two.test": "from-bbb"} {
		out, err := askTenant(t, s, tenant, "SELECT name FROM items WHERE id = 1")
		if err != nil || out != "name\n"+want+"\n(1 rows)" {
			t.Errorf("tenant %q: %q, %v; want %s", tenant, out, err, want)
		}
	}
}

func TestATenantThatDoesNotResolveIsAnErrorWithoutAnyQuery(t *testing.T) {
	s, _ := tenants(t)
	for tenant, want := range map[string]string{
		"nope.test":         "not found",
		"x' OR '1'='1":      "not found", // the name is a bound value, not part of the SQL
		"gone.test":         "not found", // its website is deleted
		"removed-host.test": "not found", // the hostname is deleted
		"dup.test":          "ambiguous",
		"managed.test":      "not supported",
		"evil.test":         "not valid",
	} {
		out, err := askTenant(t, s, tenant, "SELECT name FROM items")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("tenant %q: out %q, error %v; want an error with %q", tenant, out, err, want)
		}
	}
}

func TestAnInvalidDatabaseNameCreatesNoFile(t *testing.T) {
	s, dir := tenants(t)
	askTenant(t, s, "evil.test", "SELECT 1")
	if _, err := os.Stat(filepath.Join(dir, "bad;name.db")); err == nil {
		t.Fatal("a file named after the invalid database name was created")
	}
}

func TestTheWriteLayersStillHoldInATenantDatabase(t *testing.T) {
	s, _ := tenants(t)
	if _, err := askTenant(t, s, "one.test", "UPDATE items SET name = 'x'"); err == nil {
		t.Fatal("UPDATE was not refused")
	}
	if _, err := askTenant(t, s, "one.test", "WITH d AS (SELECT 1) DELETE FROM items"); err == nil {
		t.Fatal("a DELETE behind WITH was not refused by the database")
	}
	out, err := askTenant(t, s, "one.test", "SELECT count(*) AS n FROM items")
	if err != nil || out != "n\n2\n(1 rows)" {
		t.Fatalf("the tenant data changed: %q, %v", out, err)
	}
}

func TestTheTenantLookupCanBeReplacedWithAQueryThatReturnsOneColumn(t *testing.T) {
	s, _ := tenants(t)
	s.tenantQuery = "SELECT uuid FROM websites WHERE id = (SELECT website_id FROM hostnames WHERE fqdn = ? AND deleted_at IS NULL LIMIT 1)"
	out, err := askTenant(t, s, "two.test", "SELECT name FROM items WHERE id = 1")
	if err != nil || out != "name\nfrom-bbb\n(1 rows)" {
		t.Fatalf("%q, %v", out, err)
	}
}

func TestTheToolOffersTheTenantArgumentAndStaysSmall(t *testing.T) {
	s, _ := tenants(t)
	res, err := connect(t, s).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), `"tenant"`) {
		t.Errorf("the schema has no tenant argument: %s", raw)
	}
	if !strings.Contains(string(raw), `"check"`) {
		t.Errorf("the check tool is not listed: %s", raw)
	}
	if !strings.Contains(string(raw), `"code"`) {
		t.Errorf("the code tool is not listed: %s", raw)
	}
	if len(raw) > 1800 {
		t.Errorf("tools/list is %d bytes, the budget for this version is 1800", len(raw))
	}
}

func TestAMalformedTenantLookupFailsWithAnErrorAndFindsNothing(t *testing.T) {
	s, _ := tenants(t)
	for name, q := range map[string]string{"no placeholder": "SELECT uuid FROM websites", "two placeholders": "SELECT uuid FROM websites WHERE id = ? OR id = ?", "not a select": "DELETE FROM websites WHERE id = ?"} {
		s.tenantQuery = q
		if out, err := askTenant(t, s, "one.test", "SELECT 1"); err == nil {
			t.Errorf("%s: a lookup that does not take exactly one tenant must fail, got %q", name, out)
		}
	}
}
