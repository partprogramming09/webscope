package page

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	htmlpkg "html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxZipEntry     = 20 << 20 // bytes read from one entry of an xlsx
	defaultWaitSecs = 10
	maxWaitSecs     = 60
	maxHeaders      = 30
)

type fileMark struct {
	size int64
	mod  time.Time
}

// downloadDir is the folder where Chrome saves files: the Tool's DownloadDir, else WEBSCOPE_DOWNLOADS, else ~/Downloads.
func (p *Tool) downloadDir() string {
	if p.DownloadDir != "" {
		return p.DownloadDir
	}
	if d := os.Getenv("WEBSCOPE_DOWNLOADS"); d != "" {
		return d
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Downloads")
}

// listFiles marks every finished-looking file of dir: partial downloads and hidden files are left out.
func listFiles(dir string) (map[string]fileMark, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]fileMark{}
	for _, e := range es {
		n := e.Name()
		if e.IsDir() || strings.HasPrefix(n, ".") || strings.HasSuffix(n, ".crdownload") || strings.HasSuffix(n, ".tmp") || strings.HasSuffix(n, ".part") {
			continue
		}
		if i, err := e.Info(); err == nil && i.Mode().IsRegular() {
			out[n] = fileMark{i.Size(), i.ModTime()}
		}
	}
	return out, nil
}

// download is arm (note the files already in the downloads folder) and wait (report the file that appears after a real click, or fail).
func (p *Tool) download(args map[string]any) (string, error) {
	dir := p.downloadDir()
	if v, _ := args["arm"].(bool); v {
		seen, err := listFiles(dir)
		if err != nil {
			return "", fmt.Errorf("cannot read the downloads folder %s (%v). Next: set WEBSCOPE_DOWNLOADS to the folder Chrome saves to", dir, err)
		}
		p.mu.Lock()
		p.dlSeen, p.dlLast = seen, ""
		p.mu.Unlock()
		return fmt.Sprintf("armed: %d files already in %s are ignored. Next: make the real click in the page, then page(download, {wait:true}).", len(seen), dir), nil
	}
	if v, _ := args["report"].(bool); v {
		p.mu.Lock()
		files := slices.Clone(p.dlFiles)
		p.mu.Unlock()
		if len(files) == 0 {
			return "none: this session has not seen a file appear in " + dir + ".", nil
		}
		return fmt.Sprintf("this session saw %d file(s) appear in %s: %s. They may hold data of the app: tell the user, who can delete them (the server does not).", len(files), dir, strings.Join(files, ", ")), nil
	}
	if v, _ := args["wait"].(bool); !v {
		return "", fmt.Errorf("download needs {arm:true} before the click, {wait:true} after it, or {report:true}")
	}
	p.mu.Lock()
	seen := p.dlSeen
	p.mu.Unlock()
	if seen == nil {
		return "", fmt.Errorf("nothing armed. Next: page(download, {arm:true}), then make the real click, then wait")
	}
	secs := defaultWaitSecs
	if f, ok := args["seconds"].(float64); ok && f >= 1 && f <= maxWaitSecs {
		secs = int(f)
	}
	var cand string
	var prev fileMark
	stable := 0
	for deadline := time.Now().Add(time.Duration(secs) * time.Second); ; time.Sleep(200 * time.Millisecond) {
		now, err := listFiles(dir)
		if err == nil {
			cand = ""
			var names []string
			for n, m := range now {
				if old, ok := seen[n]; !ok || old != m {
					names = append(names, n)
				}
			}
			sort.Slice(names, func(i, j int) bool { return now[names[i]].mod.After(now[names[j]].mod) })
			if len(names) > 0 {
				cand = names[0]
				if now[cand] == prev {
					stable++
				} else {
					prev, stable = now[cand], 0
				}
				if stable >= 2 {
					return p.describeFile(filepath.Join(dir, cand), now[cand])
				}
			} else {
				stable = 0
			}
		}
		if time.Now().After(deadline) {
			break
		}
	}
	return "", fmt.Errorf("no file appeared in %s within %d s, so nothing was downloaded: do not report the export as downloaded. Next: check that the real click ran and that WEBSCOPE_DOWNLOADS is the folder Chrome saves to", dir, secs)
}

// isText is true for bytes that are text in UTF-8 or in Latin-1 (no NUL, and almost no control characters).
func isText(head []byte) bool {
	if bytes.Contains(head, []byte{0}) {
		return false
	}
	if utf8.Valid(head) {
		return true
	}
	ctl := 0
	for _, b := range head {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			ctl++
		}
	}
	return ctl*50 < len(head)+1 // under 2% control characters: Latin-1 text, as a legacy export writes it
}

var (
	htmlRow  = regexp.MustCompile(`(?is)<tr[\s>].*?(?:</tr>|$)`)
	htmlCell = regexp.MustCompile(`(?is)<t[hd][\s>].*?(?:</t[hd]>|$)`)
	htmlTag  = regexp.MustCompile(`(?s)<[^>]*>`)
)

// textSheet gives the header names and the row count of a text that is a table: an HTML table (what a legacy .xls often is) or lines with tabs, semicolons or
// commas. It is "" for any other text, and never holds a value.
func textSheet(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return ""
	}
	if len(raw) > 512<<10 {
		raw = raw[:512<<10]
	}
	text := string(raw)
	if !utf8.Valid(raw) { // Latin-1: each byte is a character
		r := make([]rune, len(raw))
		for i, b := range raw {
			r[i] = rune(b)
		}
		text = string(r)
	}
	var heads []string
	rows := 0
	if strings.Contains(strings.ToLower(text), "<table") {
		all := htmlRow.FindAllString(text, -1)
		rows = len(all)
		if rows > 0 {
			for _, c := range htmlCell.FindAllString(all[0], -1) {
				heads = append(heads, strings.TrimSpace(htmlpkg.UnescapeString(htmlTag.ReplaceAllString(c, ""))))
			}
		}
	} else {
		lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
		rows = len(lines)
		if rows > 0 {
			for _, sep := range []string{"\t", ";", ","} {
				if strings.Contains(lines[0], sep) {
					for _, h := range strings.Split(lines[0], sep) {
						heads = append(heads, strings.Trim(strings.TrimSpace(h), `"`))
					}
					break
				}
			}
		}
	}
	if len(heads) == 0 {
		return ""
	}
	if len(heads) > maxHeaders {
		heads = heads[:maxHeaders]
	}
	for i := range heads {
		heads[i] = clip(heads[i], 40)
	}
	return fmt.Sprintf("\nheaders: %s\nrows: %d (header included)", strings.Join(heads, ", "), rows)
}

// describeFile reports name, size, type by its first bytes and time, and for an xlsx the header names and the row count, never a value.
func (p *Tool) describeFile(path string, m fileMark) (string, error) {
	head := make([]byte, 512)
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	n, _ := io.ReadFull(f, head)
	f.Close()
	head = head[:n]
	kind := "unknown"
	var extra string
	switch {
	case bytes.HasPrefix(head, []byte("PK\x03\x04")):
		kind = "zip"
		if k, e := zipKind(path); k != "" {
			kind, extra = k, e
		}
	case bytes.HasPrefix(head, []byte("%PDF")):
		kind = "pdf"
	case isText(head):
		kind, extra = "text", textSheet(path)
	}
	p.mu.Lock()
	p.dlLast = filepath.Base(path)
	if note := fmt.Sprintf("%s (%d bytes)", clip(p.dlLast, 80), m.size); !slices.Contains(p.dlFiles, note) {
		p.dlFiles = append(p.dlFiles, note)
	}
	p.mu.Unlock()
	return fmt.Sprintf("downloaded: %s, %d bytes, type %s (by its first bytes), %s%s", clip(filepath.Base(path), 80), m.size, kind, m.mod.Format("15:04:05"), extra), nil
}

// zipKind says xlsx or docx by the files inside the zip; for an xlsx it adds the headers of the first sheet and its row count.
func zipKind(path string) (string, string) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", ""
	}
	defer r.Close()
	files := map[string]*zip.File{}
	var sheets []string
	for _, f := range r.File {
		files[f.Name] = f
		if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml") {
			sheets = append(sheets, f.Name)
		}
	}
	if files["word/document.xml"] != nil {
		return "docx", ""
	}
	if files["xl/workbook.xml"] == nil || len(sheets) == 0 {
		return "", ""
	}
	sort.Strings(sheets)
	shared := readShared(files["xl/sharedStrings.xml"])
	headers, rows := readSheet(files[sheets[0]], shared)
	if len(headers) > maxHeaders {
		headers = headers[:maxHeaders]
	}
	for i := range headers {
		headers[i] = clip(headers[i], 40)
	}
	return "xlsx", fmt.Sprintf("\nheaders: %s\nrows: %d (header included)", strings.Join(headers, ", "), rows)
}

// open reads one zip entry, capped.
func openEntry(f *zip.File) io.Reader {
	rc, err := f.Open()
	if err != nil {
		return strings.NewReader("")
	}
	return io.LimitReader(rc, maxZipEntry)
}

// readShared returns the shared strings of an xlsx in order.
func readShared(f *zip.File) []string {
	if f == nil {
		return nil
	}
	d := xml.NewDecoder(openEntry(f))
	var out []string
	var cur strings.Builder
	var stack []string
	for {
		tok, err := d.Token()
		if err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, t.Name.Local)
			if t.Name.Local == "si" {
				cur.Reset()
			}
		case xml.EndElement:
			if t.Name.Local == "si" {
				out = append(out, cur.String())
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 && stack[len(stack)-1] == "t" && !contains(stack, "rPh") {
				cur.Write(t)
			}
		}
	}
}

func contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

// readSheet returns the cells of the first row (their text, the header names) and how many rows the sheet has.
func readSheet(f *zip.File, shared []string) ([]string, int) {
	d := xml.NewDecoder(openEntry(f))
	var headers []string
	rows := 0
	var stack []string
	var ctype, cur string
	for {
		tok, err := d.Token()
		if err != nil {
			return headers, rows
		}
		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, t.Name.Local)
			switch t.Name.Local {
			case "row":
				rows++
			case "c":
				ctype, cur = "", ""
				for _, a := range t.Attr {
					if a.Name.Local == "t" {
						ctype = a.Value
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "c" && rows == 1 {
				v := cur
				if ctype == "s" {
					var i int
					if _, e := fmt.Sscanf(cur, "%d", &i); e == nil && i >= 0 && i < len(shared) {
						v = shared[i]
					}
				}
				headers = append(headers, v)
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if rows == 1 && len(stack) > 0 {
				if l := stack[len(stack)-1]; l == "v" || l == "t" {
					cur += string(t)
				}
			}
		}
	}
}
