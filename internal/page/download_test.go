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

func TestDownloadReportListsTheFilesTheSessionSawAppearSoTheUserCanDeleteThem(t *testing.T) {
	p, dir := dlTool(t)
	if out, err := askPage(t, p, "download", map[string]any{"report": true}); err != nil || !strings.Contains(out, "none") {
		t.Fatalf("no download yet: %q %v", out, err)
	}
	askPage(t, p, "download", map[string]any{"arm": true})
	os.WriteFile(filepath.Join(dir, "one.pdf"), []byte("%PDF-1.4"), 0o600)
	if _, err := askPage(t, p, "download", map[string]any{"wait": true, "seconds": 3}); err != nil {
		t.Fatal(err)
	}
	askPage(t, p, "download", map[string]any{"arm": true})
	writeXLSX(t, filepath.Join(dir, "two.xlsx"))
	if _, err := askPage(t, p, "download", map[string]any{"wait": true, "seconds": 3}); err != nil {
		t.Fatal(err)
	}
	out, err := askPage(t, p, "download", map[string]any{"report": true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"one.pdf", "two.xlsx", dir, "delete"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	if strings.Contains(out, "secret-value") {
		t.Errorf("a cell value shows: %q", out)
	}
}

// TestDownloadReadsTheHeadersOfATextSheet: a legacy .xls is often an HTML table or a text with tabs or semicolons, in Latin-1: its header names and row count
// come back, never a value.
func TestDownloadReadsTheHeadersOfATextSheet(t *testing.T) {
	latin1 := func(s string) []byte {
		var b []byte
		for _, r := range s {
			b = append(b, byte(r))
		}
		return b
	}
	for name, c := range map[string]struct {
		body  []byte
		heads string
		rows  string
	}{
		"html.xls":  {latin1("<html><table><tr><th>Cotización</th><th>Cliente</th></tr><tr><td>secret-value</td><td>otro</td></tr><tr><td>x</td><td>y</td></tr></table></html>"), "headers: Cotización, Cliente", "rows: 3 (header included)"},
		"tabs.xls":  {[]byte("Fecha\tTotal\nsecret-value\t5\n"), "headers: Fecha, Total", "rows: 2 (header included)"},
		"semi.csv":  {[]byte("Código;Nombre;Valor\nsecret-value;a;1\nz;b;2\n"), "headers: Código, Nombre, Valor", "rows: 3 (header included)"},
		"plain.txt": {[]byte("una sola línea sin separadores"), "", ""},
	} {
		p, dir := dlTool(t)
		askPage(t, p, "download", map[string]any{"arm": true})
		os.WriteFile(filepath.Join(dir, name), c.body, 0o600)
		out, err := askPage(t, p, "download", map[string]any{"wait": true, "seconds": 3})
		if err != nil || !strings.Contains(out, "type text") {
			t.Fatalf("%s: %q %v", name, out, err)
		}
		if strings.Contains(out, "secret-value") {
			t.Errorf("%s: a cell value shows: %q", name, out)
		}
		if c.heads != "" && (!strings.Contains(out, c.heads) || !strings.Contains(out, c.rows)) {
			t.Errorf("%s lacks %q and %q: %q", name, c.heads, c.rows, out)
		}
		if c.heads == "" && strings.Contains(out, "headers:") {
			t.Errorf("%s has no table: it must not invent headers: %q", name, out)
		}
	}
}

func TestDownloadFailureNamesThePartialFileTheNewestFileAndWhatChromeIsSetTo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	prof := filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Profile 1")
	os.MkdirAll(prof, 0o755)
	os.WriteFile(filepath.Join(prof, "Preferences"), []byte(`{"download":{"prompt_for_download":true,"default_directory":"/elsewhere/dl"},"other":{"secret":"hunter2"}}`), 0o600)
	p, dir := dlTool(t)
	os.WriteFile(filepath.Join(dir, "old.xlsx"), []byte("PK"), 0o600)
	askPage(t, p, "download", map[string]any{"arm": true})
	os.WriteFile(filepath.Join(dir, "half.xlsx.crdownload"), []byte("PK"), 0o600)
	_, err := askPage(t, p, "download", map[string]any{"wait": true, "seconds": 1})
	if err == nil {
		t.Fatal("no file must fail")
	}
	for _, want := range []string{"A partial download is there (half.xlsx.crdownload)", "The newest file there is old.xlsx", "Profile 1", "asks where to save", "saves to /elsewhere/dl"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the failure must say %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("nothing but the two download settings may be read out of Preferences: %v", err)
	}
}
