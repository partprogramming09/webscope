package page

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadAttachesTheFileToTheInputAndFiresItsEvents(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "datos.csv")
	os.WriteFile(file, []byte("a;b\n1;2\n"), 0o600)
	out, _ := runStepsTool(t, func(p *Tool, load string) []string {
		byField, err := askPage(t, p, "upload", map[string]any{"path": file, "field": "Adjunto"})
		if err != nil {
			t.Fatal(err)
		}
		alone, err := askPage(t, p, "upload", map[string]any{"path": file})
		if err != nil {
			t.Fatal(err)
		}
		emptied, err := askPage(t, p, "upload", map[string]any{"path": file})
		if err != nil {
			t.Fatal(err)
		}
		return []string{
			load,
			"(document.body.insertAdjacentHTML('beforeend','<label for=cv>Adjunto</label><input type=file id=cv style=\"display:none\">'),window.__got=[],document.getElementById('cv').addEventListener('change',async e=>{const f=e.target.files[0];window.__got.push(f.name+'|'+f.type+'|'+await f.text())}),'ok')",
			byField,
			"new Promise(r=>setTimeout(()=>r(window.__got.join(' ## ')),300))",
			alone,
			"new Promise(r=>setTimeout(()=>r(window.__got.length),300))",
			"(document.getElementById('cv').addEventListener('change',e=>{e.target.value=''}),'ok')",
			emptied,
		}
	}, nil)
	if !strings.Contains(out[2], "attached datos.csv (8 bytes) to the file input «Adjunto»; it holds 1 file(s)") {
		t.Errorf("upload by field: %q", out[2])
	}
	if !strings.HasPrefix(out[3], "datos.csv|text/csv") || !strings.Contains(out[3], "a;b\n1;2") {
		t.Errorf("the page's change handler must receive the name, the type and the bytes: %q", out[3])
	}
	if !strings.Contains(out[7], "the app emptied the input after reading it") {
		t.Errorf("an app that clears the input after reading must not be told it holds 0 files: %q", out[7])
	}
	if !strings.Contains(out[4], "attached datos.csv") || out[5] != "2" {
		t.Errorf("a page with one file input needs no name: %q, %v", out[4], out[5])
	}
}

func TestUploadRefusesWhatIsOutsideTheAllowedFolderHiddenEnvOrTooBig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WEBSCOPE_UPLOADS", dir)
	p := testPage(t, appOrigin)
	write := func(name string, size int) string {
		f := filepath.Join(dir, name)
		os.WriteFile(f, make([]byte, size), 0o600)
		return f
	}
	outside := filepath.Join(t.TempDir(), "x.txt")
	os.WriteFile(outside, []byte("x"), 0o600)
	link := filepath.Join(dir, "link.txt")
	os.Symlink(outside, link)
	for name, path := range map[string]string{
		"not under an allowed folder": outside,
		"a symlink out of the folder": link,
		"a hidden file":               write(".secret", 3),
		"too big":                     write("big.bin", maxUploadBytes+1),
		"missing":                     filepath.Join(dir, "nope.txt"),
	} {
		if _, err := askPage(t, p, "upload", map[string]any{"path": path, "field": "x"}); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if _, err := askPage(t, p, "upload", map[string]any{"field": "x"}); err == nil {
		t.Error("no path must be refused")
	}
	if _, err := askPage(t, p, "upload", map[string]any{"path": write("ok.txt", 3), "field": "x"}); err != nil {
		t.Errorf("a file in the allowed folder: %v", err)
	}
}
