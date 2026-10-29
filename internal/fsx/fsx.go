// Package fsx reads the files of a project the safe way: it never follows a symbolic link and leaves out dependencies, the files the app writes and .env files.
package fsx

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	MaxFiles = 20000   // files read at most
	MaxBytes = 2 << 20 // a bigger file is skipped
)

// SkipDirs are never walked: dependencies, files the app writes, and the repository.
var SkipDirs = map[string]bool{"vendor": true, "node_modules": true, "storage": true, ".git": true}

// Hit is one line that matched.
type Hit struct {
	File string
	Line int
	Text string
}

// Lines are the lines already read, by file.
type Lines map[string][]string

// PHPFiles are the .php files under root, read-only: symbolic links are never followed (a link could lead out of the root), .env* files, the
// folders in SkipDirs and files over MaxBytes are left out. The names come back relative to the real root.
func PHPFiles(root string) ([]string, string, error) {
	return WalkFiles(root, func(n string) bool { return strings.HasSuffix(n, ".php") })
}

// WalkFiles is the safe walk behind PHPFiles for any kind of file: want says which names to keep.
func WalkFiles(root string, want func(name string) bool) (files []string, real string, err error) {
	if real, err = filepath.EvalSymlinks(root); err != nil {
		return nil, "", err
	}
	err = filepath.WalkDir(real, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil || d.Type()&fs.ModeSymlink != 0:
			return nil
		case d.IsDir():
			if p != real && SkipDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return nil
		case !d.Type().IsRegular(): // a fifo or a socket named X.php would block the read for ever
			return nil
		case !want(d.Name()) || strings.HasPrefix(strings.ToLower(d.Name()), ".env"):
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > MaxBytes {
			return nil
		}
		if files = append(files, p); len(files) >= MaxFiles {
			return fs.SkipAll
		}
		return nil
	})
	return files, real, err
}

// Grep returns the lines of files that match re, and keeps the lines of every file it read in all.
func Grep(re *regexp.Regexp, files []string, all Lines) []Hit {
	var out []Hit
	for _, f := range files {
		ls, ok := all[f]
		if !ok {
			fh, err := os.Open(f)
			if err != nil {
				continue
			}
			sc := bufio.NewScanner(fh)
			sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
			for sc.Scan() {
				ls = append(ls, sc.Text())
			}
			fh.Close()
			all[f] = ls
		}
		for i, l := range ls {
			if re.MatchString(l) {
				out = append(out, Hit{f, i + 1, strings.TrimSpace(l)})
			}
		}
	}
	return out
}
