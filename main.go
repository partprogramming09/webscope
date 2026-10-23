// Command webscope is an MCP server over stdio. Version 0.1 has one tool, query: read-only SQL against the database in WEBSCOPE_DB_URL,
// answered as short TSV so the agent spends few tokens.
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
	version      = "0.2.0"
	instructions = `query(sql): one read-only statement (SELECT, SHOW, DESCRIBE, EXPLAIN, WITH). The answer is TSV with a header; \N is NULL; rows and cells are capped, so refine with WHERE or fewer columns. If a tool fails, report it as is.`
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
	return "Failed: " + strings.TrimRight(err.Error(), ". ") + ". Next: fix the statement or the connection (WEBSCOPE_DB_URL) and call again."
}

func newServer(s *source) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "webscope", Version: version}, &mcp.ServerOptions{Instructions: instructions})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query",
		Description: "Read-only SQL on the project database. One statement. Answer: TSV with header, capped.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, s.query)
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
	if err := newServer(&source{url: os.Getenv("WEBSCOPE_DB_URL"), tenantQuery: tenantLookup()}).Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "webscope:", err)
		os.Exit(1)
	}
}
