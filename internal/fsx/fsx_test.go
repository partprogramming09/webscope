package fsx

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

func write(t *testing.T, root, rel, text string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPHPFilesLeavesOutLinksEnvFilesSkippedFoldersAndBigFiles(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	write(t, root, "app/A.php", "<?php\n")
	write(t, root, "vendor/B.php", "<?php\n")
	write(t, root, "app/.env.php", "<?php\n")
	write(t, root, "app/big.php", string(make([]byte, MaxBytes+1)))
	write(t, root, "app/readme.md", "x")
	write(t, outside, "C.php", "<?php\n")
	if err := os.Symlink(filepath.Join(outside, "C.php"), filepath.Join(root, "app/link.php")); err != nil {
		t.Fatal(err)
	}
	files, real, err := PHPFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.Join(real, "app/A.php")}; !slices.Equal(files, want) {
		t.Errorf("got %v, want %v", files, want)
	}
}

func TestGrepReturnsMatchingLinesWithTheirNumberAndKeepsWhatItRead(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.php", "one\n  needle here  \nthree\n")
	f := filepath.Join(root, "a.php")
	all := Lines{}
	hits := Grep(regexp.MustCompile(`needle`), []string{f}, all)
	if len(hits) != 1 || hits[0] != (Hit{f, 2, "needle here"}) {
		t.Fatalf("%+v", hits)
	}
	if len(all[f]) != 3 {
		t.Errorf("the lines read were not kept: %v", all[f])
	}
}
