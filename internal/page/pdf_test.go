package page

import (
	"bytes"
	"compress/zlib"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func zlibBytes(s string) []byte {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write([]byte(s))
	w.Close()
	return b.Bytes()
}

func writePDF(t *testing.T, dir, name string, parts ...[]byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, bytes.Join(parts, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func b(s string) []byte { return []byte(s) }

func TestPDFSummaryCountsPagesNotThePagesNodeAndReadsTextAndTitle(t *testing.T) {
	dir := t.TempDir()
	path := writePDF(t, dir, "two.pdf", b("%PDF-1.4\n"+
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n"+
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>\nendobj\n"+
		"3 0 obj\n<</Type/Page/Parent 2 0 R/Contents 5 0 R>>\nendobj\n"+
		"4 0 obj\n<< /Type /Page /Parent 2 0 R >>\nendobj\n"+
		"5 0 obj\n<< /Length 40 >>\nstream\nBT /F1 12 Tf 10 10 Td (Hola) Tj ET\nendstream\nendobj\n"+
		"6 0 obj\n<< /Title (Factura \\(001\\) de venta) /Producer (x) >>\nendobj\ntrailer\n<< /Info 6 0 R >>\n%%EOF\n"))
	got := pdfSummary(path)
	for _, want := range []string{"pages: 2", "text: yes", "title: Factura (001) de venta"} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %q: %q", want, got)
		}
	}
}

func TestPDFSummaryHasNoTextForAPageOfOnlyAnImageAndNoTitleWhenThereIsNone(t *testing.T) {
	path := writePDF(t, t.TempDir(), "img.pdf", b("%PDF-1.4\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R >>\nendobj\n4 0 obj\n<< /Subtype /Image /Length 4 >>\nstream\nBTTj\nendstream\nendobj\n%%EOF\n"))
	got := pdfSummary(path)
	if !strings.Contains(got, "pages: 1") || !strings.Contains(got, "text: no") || !strings.Contains(got, "no title") {
		t.Errorf("%q", got)
	}
}

func TestPDFSummaryReadsAHexUTF16TitleAndAZlibContentStream(t *testing.T) {
	z := zlibBytes("BT (Hi) Tj ET")
	path := writePDF(t, t.TempDir(), "z.pdf", b("%PDF-1.4\n3 0 obj\n<< /Type /Page >>\nendobj\n5 0 obj\n<< /Filter /FlateDecode >>\nstream\n"), z, b("\nendstream\nendobj\n6 0 obj\n<< /Title <FEFF0048006900E9> >>\nendobj\n%%EOF\n"))
	got := pdfSummary(path)
	if !strings.Contains(got, "text: yes") || !strings.Contains(got, "title: Hié") {
		t.Errorf("%q", got)
	}
}

func TestPDFSummarySaysTitleNotReadWhenItSitsInStreamsItCannotDecode(t *testing.T) {
	// PDF 1.5: pages and Info live inside an object stream filtered with ASCII85 then Flate, which is not decoded here
	path := writePDF(t, t.TempDir(), "packed.pdf", b("%PDF-1.5\n1 0 obj\n<< /Type /ObjStm /N 3 /First 20 /Filter [/ASCII85Decode /FlateDecode] /Length 30 >>\nstream\n"), b("Gb\"0;0aGZ%Eh5Yp4&Q5CTb1VqW~>"), b("\nendstream\nendobj\n%%EOF\n"))
	got := pdfSummary(path)
	if !strings.Contains(got, "title not read") || !strings.Contains(got, "pages: not read") {
		t.Errorf("%q", got)
	}
	if strings.Contains(got, "no title") {
		t.Errorf("a title that may be in a stream it could not read is not «no title»: %q", got)
	}
}

func TestPDFSummaryReadsTheInfoOfAZlibObjectStream(t *testing.T) {
	z := zlibBytes("6 0 << /Title (Empaquetado) >>")
	path := writePDF(t, t.TempDir(), "objstm.pdf", b("%PDF-1.5\n1 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Filter /FlateDecode >>\nstream\n"), z, b("\nendstream\nendobj\n%%EOF\n"))
	if got := pdfSummary(path); !strings.Contains(got, "title: Empaquetado") {
		t.Errorf("%q", got)
	}
}

func TestPDFSummaryOfAnEncryptedFileDoesNotClaimThereIsNoText(t *testing.T) {
	path := writePDF(t, t.TempDir(), "enc.pdf", b("%PDF-1.6\n3 0 obj\n<< /Type /Page >>\nendobj\ntrailer\n<< /Encrypt 9 0 R >>\n%%EOF\n"))
	if got := pdfSummary(path); !strings.Contains(got, "text: not read") || strings.Contains(got, "no title") {
		t.Errorf("%q", got)
	}
}

func TestDownloadOfAPDFReportsPagesTextAndTitle(t *testing.T) {
	p, dir := dlTool(t)
	askPage(t, p, "download", map[string]any{"arm": true})
	writePDF(t, dir, "factura.pdf", b("%PDF-1.4\n3 0 obj\n<< /Type /Page >>\nendobj\n5 0 obj\n<< >>\nstream\nBT (x) Tj ET\nendstream\nendobj\n6 0 obj\n<< /Title (Factura 7) >>\nendobj\n%%EOF\n"))
	out, err := askPage(t, p, "download", map[string]any{"wait": true, "seconds": 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type pdf", "pages: 1", "text: yes", "title: Factura 7"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q: %q", want, out)
		}
	}
}
