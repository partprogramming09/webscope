package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/partprogramming09/webscope/internal/db"
	"github.com/partprogramming09/webscope/internal/page"
)

const (
	// Cut point for instructions and an upfront tool description: https://code.claude.com/docs/en/mcp (read 2026-11-12).
	claudeCodeDescriptionLimit = 4096
	toolsListBudget            = 1800
	// Request keys of revision 2026-07-28: https://modelcontextprotocol.io/specification/2026-07-28/changelog (read 2026-11-12).
	metaVersion      = "io.modelcontextprotocol/protocolVersion"
	metaCapabilities = "io.modelcontextprotocol/clientCapabilities"
)

func TestEveryRevisionTheSDKAnnouncesIsServedByTheTools(t *testing.T) {
	for _, v := range mcp.SupportedProtocolVersions() {
		t.Run(v, func(t *testing.T) {
			st, ct := mcp.NewInMemoryTransports()
			if _, err := newServer(fixture(t), page.New(appOrigin)).Connect(context.Background(), st, nil); err != nil {
				t.Fatal(err)
			}
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(context.Background(), ct, &mcp.ClientSessionOptions{ProtocolVersion: v})
			if err != nil {
				t.Fatalf("connect with %s: %v", v, err)
			}
			defer cs.Close()
			if got := cs.InitializeResult().ProtocolVersion; got != v {
				t.Errorf("asked for %s, the server negotiated %s", v, got)
			}
			res, err := cs.ListTools(context.Background(), nil)
			if err != nil || len(res.Tools) != 4 {
				t.Fatalf("tools under %s: %d, %v", v, len(res.Tools), err)
			}
			if text, isErr := call(t, cs, "SELECT id FROM items"); isErr || !strings.HasPrefix(text, "id\n1") {
				t.Errorf("query under %s: %q, isError=%v", v, text, isErr)
			}
		})
	}
}

// rawServer connects the server to a pipe the test writes JSON-RPC lines into, as a client over stdio would.
func rawServer(t *testing.T) (send func(string), read func() map[string]any) {
	t.Helper()
	cr, sw := io.Pipe() // server writes, test reads
	sr, cw := io.Pipe() // test writes, server reads
	if _, err := newServer(fixture(t), page.New(appOrigin)).Connect(context.Background(), &mcp.IOTransport{Reader: sr, Writer: sw}, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cw.Close(); cr.Close() })
	sc := bufio.NewScanner(cr)
	sc.Buffer(nil, 1<<20)
	send = func(line string) {
		if _, err := io.WriteString(cw, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	read = func() map[string]any {
		stop := time.AfterFunc(5*time.Second, func() { cr.Close() })
		defer stop.Stop()
		if !sc.Scan() {
			t.Fatalf("the server closed the stream: %v", sc.Err())
		}
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		m["_raw"] = len(sc.Bytes())
		return m
	}
	return
}

func TestTheOldHandshakeStillWorksOverRawJSONRPC(t *testing.T) {
	send, read := rawServer(t)
	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	res, _ := read()["result"].(map[string]any)
	if res["protocolVersion"] != "2025-06-18" || res["instructions"] == nil {
		t.Fatalf("initialize under 2025-06-18: %v", res)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	m := read()
	if tools, _ := m["result"].(map[string]any)["tools"].([]any); len(tools) != 4 {
		t.Fatalf("tools/list under 2025-06-18: %v", m)
	}
	if n := m["_raw"].(int); n > toolsListBudget {
		t.Errorf("tools/list answer is %d bytes under 2025-06-18, the budget is %d", n, toolsListBudget)
	}
}

func TestTheNewRevisionIsServedWithoutInitializeAndKeepsTheBudget(t *testing.T) {
	send, read := rawServer(t)
	meta := `"_meta":{"` + metaVersion + `":"2026-07-28","` + metaCapabilities + `":{}}`
	send(`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` + meta + `}}`)
	d, _ := read()["result"].(map[string]any)
	if d["resultType"] != "complete" || d["instructions"] == nil || d["supportedVersions"] == nil {
		t.Fatalf("server/discover: %v", d)
	}
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{` + meta + `}}`)
	m := read()
	if tools, _ := m["result"].(map[string]any)["tools"].([]any); len(tools) != 4 {
		t.Fatalf("tools/list under 2026-07-28: %v", m)
	}
	if n := m["_raw"].(int); n > toolsListBudget {
		t.Errorf("tools/list answer is %d bytes under 2026-07-28 (envelope included), the budget is %d", n, toolsListBudget)
	}
}

func TestTheNewRevisionRefusesADiscoverWithoutClientCapabilities(t *testing.T) {
	send, read := rawServer(t)
	send(`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"` + metaVersion + `":"2026-07-28"}}}`)
	e, _ := read()["error"].(map[string]any)
	if e == nil || e["code"] != float64(-32602) {
		t.Errorf("a request without clientCapabilities must be an invalid-params error: %v", e)
	}
}

func TestInstructionsAndEachDescriptionFitWhereClaudeCodeCutsThem(t *testing.T) {
	if n := utf8.RuneCountInString(instructions); n > claudeCodeDescriptionLimit {
		t.Errorf("instructions are %d characters, Claude Code cuts at %d", n, claudeCodeDescriptionLimit)
	}
	res, err := connect(t, &db.Source{}).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if n := utf8.RuneCountInString(tool.Description); n > claudeCodeDescriptionLimit {
			t.Errorf("%s description is %d characters, Claude Code cuts at %d", tool.Name, n, claudeCodeDescriptionLimit)
		}
	}
}

func TestEachToolHasATitleAndTheLocalOnesSayTheyDoNotReachTheOutsideWorld(t *testing.T) {
	res, err := connect(t, &db.Source{}).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Title == "" {
			t.Errorf("%s has no title", tool.Name)
		}
		local := tool.Name != "page"
		a := tool.Annotations
		closed := a != nil && a.OpenWorldHint != nil && !*a.OpenWorldHint
		if local && !closed {
			t.Errorf("%s only reads local files and the project database, so openWorldHint must be false", tool.Name)
		}
	}
}

func TestTheServerDoesNotAnnounceLoggingNorAListThatChanges(t *testing.T) {
	caps := connect(t, &db.Source{}).InitializeResult().Capabilities
	if caps.Logging != nil {
		t.Error("logging is deprecated in 2026-07-28 and the server sends no log messages")
	}
	if caps.Tools == nil || caps.Tools.ListChanged {
		t.Errorf("the tools are fixed: tools capability must be present without listChanged, got %+v", caps.Tools)
	}
}

func TestThePluginManifestHasTheVersionOfTheServer(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &m); err != nil || m.Version != version {
		t.Errorf("plugin.json version = %q (%v), main.go version = %q", m.Version, err, version)
	}
}

func TestTheLauncherPrintsNothingToStdoutAndNamesTheNextStepWithoutGo(t *testing.T) {
	tools := t.TempDir() // a PATH with the utilities the launcher needs and no Go
	for _, name := range []string{"dirname", "sed", "head", "mkdir", "find", "rm"} {
		p, err := exec.LookPath(name)
		if err != nil {
			t.Skip(name, "not found")
		}
		if err := os.Symlink(p, filepath.Join(tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", filepath.Join("bin", "webscope-launch"), "build")
	cmd.Env = []string{"PATH=" + tools, "CLAUDE_PLUGIN_DATA=" + t.TempDir()}
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err == nil {
		t.Fatal("without Go and without WEBSCOPE_BIN the launcher must fail")
	}
	if out.Len() != 0 {
		t.Errorf("stdout is the MCP protocol, the launcher wrote %q", out.String())
	}
	if !strings.Contains(errOut.String(), "Go is not installed") || !strings.Contains(errOut.String(), "Next: ") {
		t.Errorf("the error must name the next step: %q", errOut.String())
	}
}

func TestTheLauncherIsExecutableBecauseTheMCPConfigRunsItDirectly(t *testing.T) {
	info, err := os.Stat(filepath.Join("bin", "webscope-launch"))
	if err != nil || info.Mode()&0o111 == 0 {
		t.Errorf("bin/webscope-launch must be executable: %v, %v", info, err)
	}
}

func TestTheLauncherNamesTheNextStepForABadBinaryOrACacheThatIsNotAnAbsolutePath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(plain, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]string{
		"WEBSCOPE_BIN not executable": {"WEBSCOPE_BIN=" + plain},
		"relative WEBSCOPE_BIN":       {"WEBSCOPE_BIN=bin/webscope-launch"},
		"missing WEBSCOPE_BIN":        {"WEBSCOPE_BIN=/nonexistent/webscope"},
		"directory as WEBSCOPE_BIN":   {"WEBSCOPE_BIN=" + t.TempDir()},
		"relative cache":              {"CLAUDE_PLUGIN_DATA=relative/dir"},
		"cache is a file":             {"CLAUDE_PLUGIN_DATA=" + file},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command("sh", filepath.Join("bin", "webscope-launch"))
			cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, env...)
			var out, errOut strings.Builder
			cmd.Stdout, cmd.Stderr = &out, &errOut
			if err := cmd.Run(); err == nil {
				t.Fatal("must fail")
			}
			if out.Len() != 0 || !strings.Contains(errOut.String(), "Next: ") {
				t.Errorf("stdout %q, stderr %q: stdout must stay empty and stderr must name the next step", out.String(), errOut.String())
			}
		})
	}
}

func TestAFailedBuildStartsTheCachedBinaryAndDoesNotRetryOnEveryStart(t *testing.T) {
	tools := t.TempDir()
	for _, name := range []string{"dirname", "sed", "head", "mkdir", "find", "rm", "mv", "touch", "mktemp", "env"} {
		p, err := exec.LookPath(name)
		if err != nil {
			t.Skip(name, "not found")
		}
		if err := os.Symlink(p, filepath.Join(tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tools, "go"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	cached := filepath.Join(cache, "webscope-"+version)
	if err := os.WriteFile(cached, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(cached, old, old); err != nil {
		t.Fatal(err)
	}
	run := func() (string, string) {
		cmd := exec.Command("sh", filepath.Join("bin", "webscope-launch"), "build")
		cmd.Env = []string{"PATH=" + tools, "CLAUDE_PLUGIN_DATA=" + cache}
		var out, errOut strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("a cached binary of this version must be enough: %v, %s", err, errOut.String())
		}
		return out.String(), errOut.String()
	}
	out, errOut := run()
	if out != "" || !strings.Contains(errOut, "go build failed, using the cached") {
		t.Errorf("first start: stdout %q, stderr %q", out, errOut)
	}
	if _, errOut = run(); strings.Contains(errOut, "building") {
		t.Errorf("the second start must not build again: %q", errOut)
	}
}
