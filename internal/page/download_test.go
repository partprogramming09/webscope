package page

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeXLSX(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z := zip.NewWriter(f)
	files := map[string]string{
		"xl/workbook.xml":      `<workbook/>`,
		"xl/sharedStrings.xml": `<sst><si><t>Name</t></si><si><t>Total</t></si><si><t>secret-value</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData>` +
			`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>` +
			`<row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2"><v>5</v></c></row>` +
			`</sheetData></worksheet>`,
	}
	for n, body := range files {
		w, _ := z.Create(n)
		w.Write([]byte(body))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
}

func dlTool(t *testing.T) (*Tool, string) {
	p := testPage(t, appOrigin)
	p.DownloadDir = t.TempDir()
	return p, p.DownloadDir
}

func TestDownloadWaitNeedsArmFirst(t *testing.T) {
	p, _ := dlTool(t)
	if _, err := askPage(t, p, "download", map[string]any{"wait": true}); err == nil || !strings.Contains(err.Error(), "arm") {
		t.Fatalf("wait without arm must fail and name arm: %v", err)
	}
}

func TestDownloadReportsTheNewFileByItsBytesAndHeadersWithoutValues(t *testing.T) {
	p, dir := dlTool(t)
	os.WriteFile(filepath.Join(dir, "old.pdf"), []byte("%PDF-1.4"), 0o600)
	if out, err := askPage(t, p, "download", map[string]any{"arm": true}); err != nil || !strings.Contains(out, "armed") {
		t.Fatalf("%q %v", out, err)
	}
	writeXLSX(t, filepath.Join(dir, "export.xlsx"))
	out, err := askPage(t, p, "download", map[string]any{"wait": true, "seconds": 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"export.xlsx", "xlsx", "headers: Name, Total", "rows: 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	if strings.Contains(out, "secret-value") || strings.Contains(out, "old.pdf") {
		t.Errorf("a cell value or a file that was there before arm shows: %q", out)
	}
}

func TestDownloadTypeComesFromTheBytesNotTheName(t *testing.T) {
	p, dir := dlTool(t)
	askPage(t, p, "download", map[string]any{"arm": true})
	os.WriteFile(filepath.Join(dir, "report.dat"), []byte("%PDF-1.7 x"), 0o600)
	out, err := askPage(t, p, "download", map[string]any{"wait": true, "seconds": 3})
	if err != nil || !strings.Contains(out, "pdf") {
		t.Fatalf("%q %v", out, err)
	}
}

func TestDownloadFailsExplicitlyWhenNoFileAppears(t *testing.T) {
	p, dir := dlTool(t)
	askPage(t, p, "download", map[string]any{"arm": true})
	os.WriteFile(filepath.Join(dir, "half.xlsx.crdownload"), []byte("PK"), 0o600)
	start := time.Now()
	out, err := askPage(t, p, "download", map[string]any{"wait": true, "seconds": 1})
	if err == nil || !strings.Contains(err.Error(), "no file") {
		t.Fatalf("a partial file must not count: %q %v", out, err)
	}
	if time.Since(start) < 900*time.Millisecond {
		t.Errorf("it did not wait for the limit")
	}
}
