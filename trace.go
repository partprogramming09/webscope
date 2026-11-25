package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/partprogramming09/webscope/internal/db"
	"github.com/partprogramming09/webscope/internal/input"
)

// traceTo is where one line per call goes (stderr, which the MCP client keeps in its log); a test replaces it.
var traceTo io.Writer = os.Stderr

// traced writes one line per call of tool: its action (page, code and check have one), whether it failed and how long it took. Arguments and answers are never written.
func traced[In, Out any](tool string, h mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		start := time.Now()
		res, out, err := h(ctx, req, in)
		act := "-"
		if a, ok := any(in).(input.Args); ok && a.Action != "" {
			act = a.Action
		}
		text := resultText(res)
		status, kind := "ok", ""
		if err != nil || (res != nil && res.IsError) {
			status, kind = "failed", " err="+errKind(text)
			if text == "" && err != nil {
				kind = " err=" + errKind(err.Error())
			}
		}
		fmt.Fprintf(traceTo, "webscope: call %s %s %s %dms bytes=%d%s\n", tool, act, status, time.Since(start).Milliseconds(), len(text), kind)
		return res, out, err
	}
}

// resultText is the text of an answer, to size it and to sort a failure; it is never written.
func resultText(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

// errKind sorts the text of a failure into one fixed slug, so a log can be counted by cause without holding a value. Failures that happen in the page itself
// (a helper that cannot load) are not seen here: javascript_tool runs them in the browser.
func errKind(text string) string {
	switch {
	case db.ConnKind(text) != "":
		return db.ConnKind(text)
	case strings.Contains(text, "the call named no tenant"):
		return "tenant_missing"
	case strings.Contains(text, "was cut when it was captured"):
		return "body_cut"
	case strings.Contains(text, "unknown action") || strings.Contains(text, "is not an action of") || strings.Contains(text, "no topic "):
		return "bad_action"
	case strings.Contains(text, "is not under an allowed folder"):
		return "upload_path"
	case strings.Contains(text, "WEBSCOPE_") && strings.Contains(text, " is not set"):
		return "env"
	}
	return "other"
}

// stamped puts the time at the start of each line it writes to w.
type stamped struct{ w io.Writer }

func (s stamped) Write(p []byte) (int, error) {
	if _, err := fmt.Fprintf(s.w, "%s %s", time.Now().UTC().Format("2006-01-02T15:04:05Z"), p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// openTrace is where the lines of calls and events go: stderr, and also the file WEBSCOPE_ACTIONLOG names (appended) when it is set and can be opened.
func openTrace() io.Writer {
	path := os.Getenv("WEBSCOPE_ACTIONLOG")
	if path == "" {
		return os.Stderr
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "webscope: cannot open WEBSCOPE_ACTIONLOG %s (%v)\n", path, err)
		return os.Stderr
	}
	return io.MultiWriter(os.Stderr, stamped{f})
}
