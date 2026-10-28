// Command webscope is an MCP server over stdio. Two tools: query, read-only SQL against the database in WEBSCOPE_DB_URL answered as short TSV,
// and page, snippets for testing a screen in Chrome (WEBSCOPE_APP_URL) so the agent spends few tokens.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	version      = "0.5.16"
	instructions = `query(sql): one read-only statement (SELECT, SHOW, DESCRIBE, EXPLAIN, WITH). The answer is TSV with a header; \N is NULL; rows and cells are capped, so refine with WHERE or fewer columns. page(action,args?): test a screen in Chrome; start with page(help). check(sent,table,where,map,after?,at?,tenant?): sent is a request page captured ("POST /api/quotes"), where {column:value} picks one row, map {sent field:column}, after a later captured read; only differences are shown. code(action,args?): fields {table,tenant?} is the sheet of a table (counts, never values); consumers {table,column} lists who reads and writes a column in the backend (an estimate); map {module,walkthrough?} the routes, calls and screens of a front module; map_back {query} from them to endpoint, controller, service and table; snap {name,sql,key?,tenant?} keeps a query and snap_diff {name} shows what changed since (new, deleted, before → after). If a tool fails, report it as is.`
)

// query answers a query call: plain text on success, and on failure a text that starts with "Failed:" and names the next step, marked as an error.
func (s *source) query(ctx context.Context, _ *mcp.CallToolRequest, in queryArgs) (*mcp.CallToolResult, any, error) {
	text, err := s.answer(ctx, in)
	if err != nil {
		text = failure(err)
	}
	return &mcp.CallToolResult{IsError: err != nil, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}

// failure is the text of a failed call: what happened and what to do next.
func failure(err error) string {
	next := "fix the statement or the connection (WEBSCOPE_DB_URL) and call again."
	switch msg := err.Error(); {
	case strings.Contains(msg, "Error 1054") || strings.Contains(msg, "no such column"):
		next = "that column does not exist: code(fields {table}) or SHOW COLUMNS FROM table lists the real ones; then call again."
	case strings.Contains(msg, "Error 1146") || strings.Contains(msg, "no such table"):
		next = "that table does not exist: SHOW TABLES LIKE '%word%' lists the tables; then call again."
	}
	return "Failed: " + strings.TrimRight(err.Error(), ". ") + ". Next: " + next
}

func newServer(s *source, p *pageTool) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "webscope", Version: version}, &mcp.ServerOptions{Instructions: instructions})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query",
		Description: "Read-only SQL on the project database. One statement. Answer: TSV with header, capped.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, guarded(s.query))
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "page",
		Description: "Test a screen in Chrome (Claude in Chrome): returns text or JavaScript to run with javascript_tool. page(help) lists the actions.",
	}, guarded(p.tool))
	cd := &coder{s: s, backend: os.Getenv("WEBSCOPE_BACKEND"), front: os.Getenv("WEBSCOPE_FRONT")}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "code",
		Description: "Read the project's code: fields, consumers, map, map_back.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: codeSchema,
	}, guarded(cd.tool))
	ck := &checker{s: s, p: p}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "check",
		Description: "Saved row vs what the screen sent and read back.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: checkSchema,
	}, guarded(ck.tool))
	return srv
}

// tenantLookup is the query that finds a tenant's database: WEBSCOPE_TENANT_QUERY or Hyn Tenancy's.
func tenantLookup() string {
	if q := os.Getenv("WEBSCOPE_TENANT_QUERY"); q != "" {
		return q
	}
	return hynTenantQuery
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Fprintf(os.Stderr, "webscope %s\n", version)
	page := newPage(os.Getenv("WEBSCOPE_APP_URL"))
	page.life = ctx
	if err := newServer(&source{url: os.Getenv("WEBSCOPE_DB_URL"), tenantQuery: tenantLookup()}, page).Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "webscope:", err)
		os.Exit(1)
	}
}

// guarded is the handler h that answers an error instead of panicking: a bug in one tool must not take the whole server down.
func guarded[In, Out any](h mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (res *mcp.CallToolResult, out Out, err error) {
		defer func() {
			if r := recover(); r != nil {
				res = &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Failed: internal error (%v). Next: report it with the arguments you sent.", r)}}}
				err = nil
			}
		}()
		return h(ctx, req, in)
	}
}
