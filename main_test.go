package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/partprogramming09/webscope/internal/db"
)

// fixture writes a SQLite file with one row in items and returns a source that opens it.
func fixture(t *testing.T) *db.Source {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, q := range []string{"CREATE TABLE items (id INTEGER PRIMARY KEY)", "INSERT INTO items VALUES (1)"} {
		if _, err := conn.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return &db.Source{URL: "sqlite://" + path}
}

func connect(t *testing.T, s *db.Source) *mcp.ClientSession {
	t.Helper()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := newServer(s, newPage(appOrigin)).Connect(context.Background(), st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestOnlyQueryPageCheckAndCodeAreOffered(t *testing.T) {
	res, err := connect(t, &db.Source{}).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	tools := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		tools[tool.Name] = tool
	}
	if len(tools) != 4 || tools["query"] == nil || tools["page"] == nil || tools["check"] == nil || tools["code"] == nil {
		t.Fatalf("tools = %v", res.Tools)
	}
	if a := tools["query"].Annotations; a == nil || !a.ReadOnlyHint {
		t.Error("query is not marked read-only")
	}
	if a := tools["check"].Annotations; a == nil || !a.ReadOnlyHint {
		t.Error("check is not marked read-only")
	}
	if a := tools["code"].Annotations; a == nil || !a.ReadOnlyHint {
		t.Error("code is not marked read-only")
	}
	// page is not read-only: the snippets it hands out can fill and click in the app (writes go through the screen, ROADMAP rule 1).
	if a := tools["page"].Annotations; a != nil && a.ReadOnlyHint {
		t.Error("page is marked read-only")
	}
}

func call(t *testing.T, cs *mcp.ClientSession, sqlText string) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "query", Arguments: map[string]any{"sql": sqlText}})
	if err != nil {
		t.Fatal(err)
	}
	return res.Content[0].(*mcp.TextContent).Text, res.IsError
}

func TestAnAnswerIsPlainTextAndAFailureIsMarkedAndNamesTheNextStep(t *testing.T) {
	cs := connect(t, fixture(t))
	if text, isErr := call(t, cs, "SELECT id FROM items WHERE id = 1"); isErr || text != "id\n1\n(1 rows)" {
		t.Errorf("answer = %q, isError = %v", text, isErr)
	}
	text, isErr := call(t, cs, "DELETE FROM items")
	if !isErr || !strings.HasPrefix(text, "Failed: ") || !strings.Contains(text, "Next: ") {
		t.Errorf("failure = %q, isError = %v", text, isErr)
	}
}

func TestAToolThatPanicsAnswersAnErrorInsteadOfTakingTheServerDown(t *testing.T) {
	h := guarded(func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		var s []int
		_ = s[len(s)-1]
		return nil, nil, nil
	})
	res, _, err := h(context.Background(), nil, struct{}{})
	if err != nil || res == nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Failed") {
		t.Errorf("%+v, %v", res, err)
	}
}

func TestTheToolOffersTheTenantArgumentAndStaysSmall(t *testing.T) {
	res, err := connect(t, &db.Source{}).ListTools(context.Background(), nil)
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
