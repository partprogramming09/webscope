package page

import (
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
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

// logLines are the last error lines of the backend log, oldest first, each cut short: the lines written since the last click or key
// when the helper marked one, else the end of the file. It is the log's own text, so it can hold data. Empty without a log or an error.
func (p *Tool) logLines() string {
	p.mu.Lock()
	from, marked := p.logFrom, p.logMarked
	p.mu.Unlock()
	if !marked {
		from = -1
	}
	return p.logLinesFrom(from)
}

// logLinesFrom is logLines for the bytes after offset from; a negative offset, or one past the end (the log was rotated), reads the end of the file.
func (p *Tool) logLinesFrom(from int64) string {
	path := p.logPath()
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	size := int64(0)
	if i, err := f.Stat(); err == nil {
		size = i.Size()
	}
	start := size - logTail
	if from >= 0 && from <= size && from > start {
		start = from
	}
	if start < 0 {
		start = 0
	}
	f.Seek(start, io.SeekStart)
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

// logMark notes where the log ends now and answers that offset: what is written after it belongs to the action that follows.
func (p *Tool) logMark(w http.ResponseWriter, _ *http.Request) {
	size := int64(0)
	if path := p.logPath(); path != "" {
		if i, err := os.Stat(path); err == nil {
			size = i.Size()
		}
	}
	p.mu.Lock()
	p.logFrom, p.logMarked = size, p.logPath() != ""
	p.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, strconv.FormatInt(size, 10))
}

// noteMark keeps the size of the backend log as the place of mark p.seq, the first time that mark is given: a later payloads with the same
// mark may come after the action it was meant to precede. The caller holds p.mu. Nothing is kept without a log.
func (p *Tool) noteMark() {
	path := p.logPath()
	if path == "" {
		return
	}
	if _, ok := p.logAt[p.seq]; ok {
		return
	}
	size := int64(0)
	if i, err := os.Stat(path); err == nil {
		size = i.Size()
	}
	if p.logAt == nil {
		p.logAt = map[int]int64{}
	}
	p.logAt[p.seq] = size
}

// logRoute answers the helper with the error lines written after the offset in the query (from), or the end of the file without one.
func (p *Tool) logRoute(w http.ResponseWriter, r *http.Request) {
	from, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	if err != nil {
		from = -1
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, p.logLinesFrom(from))
}
