package code

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/partprogramming09/webscope/internal/db"
	"github.com/partprogramming09/webscope/internal/input"
)

func doctorEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"WEBSCOPE_APP_URL", "WEBSCOPE_UPLOADS", "WEBSCOPE_DOWNLOADS", "WEBSCOPE_LOG"} {
		t.Setenv(k, "")
	}
}

func runDoctor(t *testing.T, c *Tool) string {
	t.Helper()
	out, err := c.run(context.Background(), input.Args{Action: "doctor"})
	if err != nil {
		t.Fatalf("doctor must answer, not fail: %v", err)
	}
	return out
}

func TestDoctorWithAnEmptyEnvironmentListsEveryMissingVariableInOneAnswer(t *testing.T) {
	doctorEnv(t)
	out := runDoctor(t, &Tool{Src: &db.Source{}})
	for _, want := range []string{"WEBSCOPE_DB_URL: MISSING", "WEBSCOPE_APP_URL: MISSING", "WEBSCOPE_BACKEND: not set", "WEBSCOPE_FRONT: not set", "WEBSCOPE_LEGACY: not set", "Next: "} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q in:\n%s", want, out)
		}
	}
}

func TestDoctorWithAWorkingSetupSaysEachPartIsOkAndNeverPrintsAPassword(t *testing.T) {
	doctorEnv(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "app.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	t.Setenv("WEBSCOPE_APP_URL", "http://localhost:5173")
	c := &Tool{Src: &db.Source{URL: "sqlite://" + dbPath}, Backend: dir, Front: dir}
	out := runDoctor(t, c)
	for _, want := range []string{"WEBSCOPE_DB_URL: ok", "WEBSCOPE_APP_URL: ok", "WEBSCOPE_BACKEND: ok", "WEBSCOPE_FRONT: ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "FAIL") || strings.Contains(out, "MISSING") {
		t.Errorf("a working setup has no FAIL or MISSING:\n%s", out)
	}
}

func TestDoctorWithTheDatabaseDownNamesTheCauseAndHidesThePassword(t *testing.T) {
	doctorEnv(t)
	c := &Tool{Src: &db.Source{URL: "mysql://ro:s3cr3tpass@127.0.0.1:1/app"}}
	out := runDoctor(t, c)
	if !strings.Contains(out, "WEBSCOPE_DB_URL: FAIL") || !strings.Contains(out, "not reachable") {
		t.Errorf("the dead port must read FAIL and not reachable:\n%s", out)
	}
	if strings.Contains(out, "s3cr3tpass") {
		t.Errorf("the password leaked:\n%s", out)
	}
}

func TestDoctorFlagsAFolderThatDoesNotExistAndABadAppURL(t *testing.T) {
	doctorEnv(t)
	t.Setenv("WEBSCOPE_APP_URL", "localhost:5173")
	t.Setenv("WEBSCOPE_UPLOADS", filepath.Join(t.TempDir(), "nope"))
	missing := filepath.Join(t.TempDir(), "gone")
	out := runDoctor(t, &Tool{Src: &db.Source{}, Backend: missing})
	for _, want := range []string{"WEBSCOPE_BACKEND: FAIL", "WEBSCOPE_APP_URL: FAIL", "WEBSCOPE_UPLOADS: FAIL"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q in:\n%s", want, out)
		}
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("doctor must not create folders")
	}
}

func TestCodeWithTheActionOfAnotherToolNamesThatTool(t *testing.T) {
	c := &Tool{Src: &db.Source{}}
	for action, want := range map[string]string{"query": "query(sql)", "sql": "query(sql)", "state": "page(state)", "click": "page(click)", "load": "page(load)", "check": "check(sent"} {
		res, _, _ := c.Handle(context.Background(), nil, input.Args{Action: action})
		got := res.Content[0].(*mcp.TextContent).Text
		if !res.IsError || !strings.Contains(got, "Next: ") || !strings.Contains(got, want) || strings.Count(got, "Next:") != 1 {
			t.Errorf("%s: %q", action, got)
		}
	}
}

func TestCodeKeepsTheNextThatAnErrorAlreadyHas(t *testing.T) {
	c := &Tool{Src: &db.Source{}}
	res, _, _ := c.Handle(context.Background(), nil, input.Args{Action: "help", Args: map[string]any{"topic": "zzz"}})
	if got := res.Content[0].(*mcp.TextContent).Text; strings.Count(got, "Next:") != 1 {
		t.Errorf("one Next expected: %q", got)
	}
}
