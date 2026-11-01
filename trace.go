package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		status := "ok"
		if err != nil || (res != nil && res.IsError) {
			status = "failed"
		}
		fmt.Fprintf(traceTo, "webscope: call %s %s %s %dms\n", tool, act, status, time.Since(start).Milliseconds())
		return res, out, err
	}
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
