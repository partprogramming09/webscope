package page

import (
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

const (
	logTail     = 64 << 10 // bytes read from the end of the log
	logLinesMax = 3        // error lines returned
	logLineClip = 200      // bytes of each line returned
)

var logError = regexp.MustCompile(`(?i)fatal|exception|error|panic|stack trace|uncaught`)

// logPath is the backend log: the Tool's LogPath, else WEBSCOPE_LOG; empty when none is set.
func (p *Tool) logPath() string {
	if p.LogPath != "" {
		return p.LogPath
	}
	return os.Getenv("WEBSCOPE_LOG")
}

// logLines are the last error lines of the backend log, oldest first, each cut short; it is the log's own text, so it can hold data. Empty without a log or an error.
func (p *Tool) logLines() string {
	path := p.logPath()
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if i, err := f.Stat(); err == nil && i.Size() > logTail {
		f.Seek(i.Size()-logTail, io.SeekStart)
	}
	raw, _ := io.ReadAll(io.LimitReader(f, logTail))
	all := strings.Split(string(raw), "\n")
	var out []string
	for i := len(all) - 1; i >= 0 && len(out) < logLinesMax; i-- {
		if l := strings.TrimSpace(all[i]); l != "" && logError.MatchString(l) {
			out = append([]string{clip(oneLine(l), logLineClip)}, out...)
		}
	}
	return strings.Join(out, "\n")
}

// logRoute answers the helper with logLines.
func (p *Tool) logRoute(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, p.logLines())
}
