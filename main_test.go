package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connect(t *testing.T, s *source) *mcp.ClientSession {
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
	res, err := connect(t, &source{}).ListTools(context.Background(), nil)
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

func TestFailureSaysHowToSeeTheRealColumnsAndTables(t *testing.T) {
	for _, tc := range []struct{ err, want string }{
		{"Error 1054 (42S22): Unknown column 'pantalla' in 'field list'", "code(fields"},
		{"SQL logic error: no such column: pantalla (1)", "code(fields"},
		{"Error 1146 (42S02): Table 'db.nada' doesn't exist", "SHOW TABLES"},
		{"SQL logic error: no such table: nada (1)", "SHOW TABLES"},
		{"Error 1064 (42000): You have an error in your SQL syntax", "fix the statement"},
	} {
		got := failure(errors.New(tc.err), "")
		if !strings.HasPrefix(got, "Failed: "+tc.err) || !strings.Contains(got, "Next: ") || !strings.Contains(got, tc.want) {
			t.Errorf("%s: %q lacks %q", tc.err, got, tc.want)
		}
	}
}

func TestFailureOfAMissingTableWithATenantSaysACentralTableIsReadWithoutIt(t *testing.T) {
	for _, e := range []string{"Error 1146 (42S02): Table 'db.customers' doesn't exist", "SQL logic error: no such table: customers (1)"} {
		with := failure(errors.New(e), "demo.localhost")
		if !strings.Contains(with, "without tenant") || !strings.Contains(with, "SHOW TABLES") {
			t.Errorf("%s with a tenant: %q", e, with)
		}
		if without := failure(errors.New(e), ""); strings.Contains(without, "tenant") {
			t.Errorf("%s without a tenant must not mention it: %q", e, without)
		}
	}
	if got := failure(errors.New("Error 1054 (42S22): Unknown column 'x' in 'field list'"), "demo.localhost"); strings.Contains(got, "without tenant") {
		t.Errorf("a missing column is not a central table: %q", got)
	}
}

func TestFailureTextHasNoDoubledPeriodWhenTheErrorEndsInOne(t *testing.T) {
	got := failure(errors.New("Error 1792 (25006): Cannot execute statement in a READ ONLY transaction."), "")
	if strings.Contains(got, "..") {
		t.Fatalf("doubled period in %q", got)
	}
	if !strings.HasPrefix(got, "Failed: Error 1792") || !strings.Contains(got, "transaction. Next: ") {
		t.Fatalf("unexpected text %q", got)
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
