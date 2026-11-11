package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/partprogramming09/webscope/internal/db"
	"github.com/partprogramming09/webscope/internal/input"
	"github.com/partprogramming09/webscope/internal/page"
)

const appOrigin = "http://localhost:5173"

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
	if _, err := newServer(s, page.New(appOrigin)).Connect(context.Background(), st, nil); err != nil {
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
	// page is not read-only: the snippets it hands out can fill and click in the app (writes go through the screen).
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

func TestPageAcceptsTheArgumentsAtTheTopLevelToo(t *testing.T) {
	res, err := connect(t, &db.Source{}).CallTool(context.Background(), &mcp.CallToolParams{Name: "page", Arguments: map[string]any{"action": "fill", "Nombre": "Ana"}})
	if err != nil || res.IsError {
		t.Fatalf("the schema refused a top-level argument: %v %+v", err, res)
	}
	if got := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(got, `.fill({"Nombre":"Ana"})`) {
		t.Errorf("the top-level argument did not reach the snippet: %q", got)
	}
	var in input.Args
	if err := json.Unmarshal([]byte(`{"action":"fill","Nombre":"x","args":{"Nombre":"Ana"}}`), &in); err != nil || in.Args["Nombre"] != "Ana" {
		t.Errorf("a key inside args must win: %+v, %v", in, err)
	}
}

func TestACallLeavesOneLineWithTheToolTheActionAndTheResultButNoArgument(t *testing.T) {
	var buf strings.Builder
	old := traceTo
	traceTo = &buf
	defer func() { traceTo = old }()
	h := traced("page", func(context.Context, *mcp.CallToolRequest, input.Args) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Failed: Ana"}}}, nil, nil
	})
	h(context.Background(), nil, input.Args{Action: "fill", Args: map[string]any{"Nombre": "Ana"}})
	line := buf.String()
	if !strings.HasPrefix(line, "webscope: call page fill failed ") || strings.Contains(line, "Ana") || strings.Count(line, "\n") != 1 {
		t.Errorf("one line with tool, action and result, and no value: %q", line)
	}
}

func TestTheActionLogFileGetsTheLinesWithATimeWhenItIsSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actions.log")
	t.Setenv("WEBSCOPE_ACTIONLOG", path)
	w := openTrace()
	fmt.Fprintln(w, "webscope: call page click ok 3ms")
	raw, err := os.ReadFile(path)
	if err != nil || !regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ webscope: call page click ok 3ms\n$`).Match(raw) {
		t.Errorf("the file must hold the line with a time: %q, %v", raw, err)
	}
	t.Setenv("WEBSCOPE_ACTIONLOG", "")
	if openTrace() != os.Stderr {
		t.Errorf("without the variable only stderr is used")
	}
}
