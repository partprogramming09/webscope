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

func TestFailureTextHasNoDoubledPeriodWhenTheErrorEndsInOne(t *testing.T) {
	got := failure(errors.New("Error 1792 (25006): Cannot execute statement in a READ ONLY transaction."))
	if strings.Contains(got, "..") {
		t.Fatalf("doubled period in %q", got)
	}
	if !strings.HasPrefix(got, "Failed: Error 1792") || !strings.Contains(got, "transaction. Next: ") {
		t.Fatalf("unexpected text %q", got)
	}
}
