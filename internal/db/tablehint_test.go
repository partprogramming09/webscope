package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func tablesFixture(t *testing.T) *Source {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		"CREATE TABLE wm_activity (id INTEGER)",
		"CREATE TABLE wm_tasks_old (id INTEGER)",
		"CREATE TABLE tm_do (id INTEGER)",
		"CREATE TABLE tm_clients (id INTEGER)",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return &Source{URL: "sqlite://" + path}
}

func TestAMissingTableSuggestsTheNearestOneAndListsTheOnesWithItsPrefix(t *testing.T) {
	s := tablesFixture(t)
	for _, tc := range []struct {
		sql  string
		want []string
		not  []string
	}{
		{"SELECT * FROM tm_client", []string{"Did you mean «tm_clients»?", "Tables with prefix «tm_»: "}, nil},
		{"SELECT * FROM wm_tasks", []string{"Did you mean «wm_tasks_old»?", "wm_activity", "wm_tasks_old"}, []string{"tm_do"}},
		{"SELECT * FROM zz_nothing", []string{"no such table"}, []string{"Did you mean", "Tables with prefix"}},
	} {
		_, err := ask(t, s, tc.sql, 0)
		if err == nil {
			t.Errorf("%s: no error", tc.sql)
			continue
		}
		for _, w := range tc.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: %q lacks %q", tc.sql, err, w)
			}
		}
		for _, n := range tc.not {
			if strings.Contains(err.Error(), n) {
				t.Errorf("%s: %q has %q", tc.sql, err, n)
			}
		}
	}
}
