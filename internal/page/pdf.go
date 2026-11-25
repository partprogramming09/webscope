package page

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	pdfMaxFile    = 16 << 20 // bigger files are not read
	pdfMaxStream  = 4 << 20  // inflated bytes kept of one stream
	pdfMaxInflate = 32 << 20 // inflated bytes kept of the whole file
	pdfMaxStreams = 3000
)

var (
	pdfPageRe   = regexp.MustCompile(`/Type\s*/Page`)
	pdfFilterRe = regexp.MustCompile(`/Filter\s*(\[[^\]]*\]|/\w+)`)
	pdfNameRe   = regexp.MustCompile(`/(\w+)`)
	pdfTextRe   = regexp.MustCompile(`(?s)\bBT\b.*?[)>\]]\s*T[jJ]\b`)
	pdfTitleRe  = regexp.MustCompile(`/Title\s*(\((?:\\.|[^\\()]|\((?:\\.|[^\\()])*\))*\)|<[0-9A-Fa-f\s]*>)`)
)

// pdfSummary says how many pages a PDF has, whether it has text and its title, from the bytes alone (standard library, no PDF parser).
// It reads the file's own objects and inflates the Flate streams; what sits in a stream it cannot decode (another filter, encryption) is
// said as «not read», never as a zero or an absence.
func pdfSummary(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, _ := io.ReadAll(io.LimitReader(f, pdfMaxFile+1))
	if len(data) > pdfMaxFile {
		return "\npages, text and title not read: the file is over 16 MiB"
	}
	outside, decoded, unreadPacked, unreadContent := pdfParts(data)
	whole := append([][]byte{outside}, decoded...)
	encrypted := bytes.Contains(outside, []byte("/Encrypt"))

	pages := 0
	for _, part := range whole {
		pages += pdfCountPages(part)
	}
	var pg string
	switch {
	case unreadPacked && pages == 0:
		pg = "not read"
	case unreadPacked:
		pg = fmt.Sprintf("at least %d", pages)
	default:
		pg = strconv.Itoa(pages)
	}

	text := "no"
	for _, part := range whole {
		if pdfTextRe.Match(part) {
			text = "yes"
			break
		}
	}
	if text == "no" && (encrypted || unreadContent || unreadPacked) {
		text = "not read"
	}

	title := ""
	for _, part := range whole {
		if title = pdfTitle(part); title != "" {
			break
		}
	}
	switch {
	case title != "":
		title = "title: " + title
	case encrypted || unreadPacked:
		title = "title not read"
	default:
		title = "no title"
	}
	return fmt.Sprintf("\npages: %s, text: %s, %s", pg, text, title)
}

// pdfParts splits the file into what lies outside the streams, the decoded body of each stream that holds objects or page content, and
// whether an object stream or a content stream was left undecoded.
func pdfParts(data []byte) (outside []byte, decoded [][]byte, unreadPacked, unreadContent bool) {
	var out bytes.Buffer
	pos, budget, count := 0, pdfMaxInflate, 0
	kw := []byte("stream")
	for {
		i := bytes.Index(data[pos:], kw)
		if i < 0 {
			break
		}
		i += pos
		// «stream» opens a body only after a dictionary and before a line break; «endstream» ends one
		start := i + len(kw)
		if i >= 3 && string(data[i-3:i]) == "end" || start >= len(data) || (data[start] != '\r' && data[start] != '\n') || !bytes.HasSuffix(bytes.TrimRight(data[max(0, i-8):i], " \r\n\t"), []byte(">>")) {
			out.Write(data[pos:start])
			pos = start
			continue
		}
		if data[start] == '\r' {
			start++
		}
		if start < len(data) && data[start] == '\n' {
			start++
		}
		end := bytes.Index(data[start:], []byte("endstream"))
		if end < 0 {
			end = len(data) - start
		}
		end += start
		dictFrom := max(0, i-1500)
		if k := bytes.LastIndex(data[dictFrom:i], []byte("obj")); k >= 0 {
			dictFrom += k
		}
		dict := data[dictFrom:i]
		out.Write(data[pos:i])
		pos = end
		count++
		packed := bytes.Contains(dict, []byte("/ObjStm"))
		hasType := bytes.Contains(dict, []byte("/Type")) || bytes.Contains(dict, []byte("/Subtype"))
		content := !hasType || bytes.Contains(dict, []byte("/Form"))
		if !packed && !content {
			continue // an image, a font, a cross-reference table: nothing about pages, text or title
		}
		body := bytes.TrimRight(data[start:end], "\r\n")
		names := pdfFilters(dict)
		switch {
		case len(names) == 0:
			decoded = append(decoded, body)
		case len(names) == 1 && (names[0] == "FlateDecode" || names[0] == "Fl") && count <= pdfMaxStreams && budget > 0:
			if b, ok := inflate(body, min(pdfMaxStream, budget)); ok {
				budget -= len(b)
				decoded = append(decoded, b)
			} else if packed {
				unreadPacked = true
			} else {
				unreadContent = true
			}
		case packed:
			unreadPacked = true
		default:
			unreadContent = true
		}
	}
	out.Write(data[pos:])
	return out.Bytes(), decoded, unreadPacked, unreadContent
}

func pdfFilters(dict []byte) []string {
	m := pdfFilterRe.FindSubmatch(dict)
	if m == nil {
		return nil
	}
	var names []string
	for _, n := range pdfNameRe.FindAllSubmatch(m[1], -1) {
		if string(n[1]) != "Filter" {
			names = append(names, string(n[1]))
		}
	}
	return names
}

// inflate returns the zlib-inflated body, up to limit bytes; a stream that does not inflate at all is not ok.
func inflate(body []byte, limit int) ([]byte, bool) {
	r, err := zlib.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, false
	}
	defer r.Close()
	b, _ := io.ReadAll(io.LimitReader(r, int64(limit)))
	return b, len(b) > 0
}

// pdfCountPages counts «/Type /Page» that is not «/Type /Pages» (nor /PageLabels): the character after the name must not be a letter.
func pdfCountPages(b []byte) int {
	n := 0
	for _, m := range pdfPageRe.FindAllIndex(b, -1) {
		if m[1] < len(b) {
			c := b[m[1]]
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
				continue
			}
		}
		n++
	}
	return n
}

// pdfTitle is the first /Title that belongs to the document (an outline entry has a /Parent, /Dest or /Next next to it), decoded and short.
func pdfTitle(b []byte) string {
	for _, m := range pdfTitleRe.FindAllSubmatchIndex(b, -1) {
		from, to := max(0, m[0]-300), min(len(b), m[1]+300)
		if k := bytes.LastIndex(b[from:m[0]], []byte("obj")); k >= 0 {
			from += k
		}
		if k := bytes.Index(b[m[1]:to], []byte("endobj")); k >= 0 {
			to = m[1] + k
		}
		near := b[from:to]
		if bytes.Contains(near, []byte("/Parent")) || bytes.Contains(near, []byte("/Dest")) || bytes.Contains(near, []byte("/Next")) || bytes.Contains(near, []byte("/Prev")) {
			continue
		}
		if t := strings.Join(strings.Fields(pdfString(b[m[2]:m[3]])), " "); t != "" {
			return clip(t, 80)
		}
	}
	return ""
}

// pdfString decodes a literal «(…)» or hexadecimal «<…>» PDF string: UTF-16 with a byte order mark, else UTF-8 if valid, else Latin-1.
func pdfString(s []byte) string {
	var raw []byte
	if s[0] == '<' {
		h := strings.Join(strings.Fields(string(s[1:len(s)-1])), "")
		if len(h)%2 == 1 {
			h += "0"
		}
		for i := 0; i+1 < len(h); i += 2 {
			v, _ := strconv.ParseUint(h[i:i+2], 16, 8)
			raw = append(raw, byte(v))
		}
	} else {
		in := s[1 : len(s)-1]
		for i := 0; i < len(in); i++ {
			c := in[i]
			if c != '\\' || i+1 >= len(in) {
				raw = append(raw, c)
				continue
			}
			i++
			switch e := in[i]; {
			case e == 'n':
				raw = append(raw, '\n')
			case e == 'r':
				raw = append(raw, '\r')
			case e == 't':
				raw = append(raw, '\t')
			case e == 'b':
				raw = append(raw, '\b')
			case e == 'f':
				raw = append(raw, '\f')
			case e >= '0' && e <= '7':
				v, j := int(e-'0'), 1
				for ; j < 3 && i+1 < len(in) && in[i+1] >= '0' && in[i+1] <= '7'; j++ {
					i++
					v = v*8 + int(in[i]-'0')
				}
				raw = append(raw, byte(v))
			case e == '\n' || e == '\r':
				// a line continuation
			default:
				raw = append(raw, e)
			}
		}
	}
	switch {
	case len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF:
		return utf16Decode(raw[2:], true)
	case len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE:
		return utf16Decode(raw[2:], false)
	case len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF:
		return string(raw[3:])
	case utf8.Valid(raw):
		return string(raw)
	}
	rs := make([]rune, len(raw))
	for i, c := range raw {
		rs[i] = rune(c)
	}
	return string(rs)
}

func utf16Decode(b []byte, bigEndian bool) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if bigEndian {
			u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			u = append(u, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	return string(utf16.Decode(u))
}
