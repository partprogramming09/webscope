package page

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxUploadBytes = 5 << 20 // a file bigger than this is refused

type upload struct {
	name, typ string
	data      []byte
}

// uploadRoots are the folders a file may be read from: WEBSCOPE_UPLOADS (several separated by commas), else the system temp folder and /tmp.
func uploadRoots() []string {
	var roots []string
	if v := os.Getenv("WEBSCOPE_UPLOADS"); v != "" {
		roots = strings.Split(v, ",")
	} else {
		roots = []string{os.TempDir(), "/tmp"}
	}
	var out []string
	for _, r := range roots {
		if r = strings.TrimSpace(r); r == "" {
			continue
		}
		if abs, err := filepath.EvalSymlinks(r); err == nil {
			out = append(out, abs)
		}
	}
	return out
}

// readUpload reads the file at path if it sits under an allowed folder, is a regular file of at most maxUploadBytes, and is neither hidden nor an .env file.
func readUpload(path string) (*upload, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s (%v)", path, err)
	}
	ok := false
	for _, r := range uploadRoots() {
		if rel, err := filepath.Rel(r, real); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			ok = true
		}
	}
	if !ok {
		return nil, fmt.Errorf("%s is not under an allowed folder (WEBSCOPE_UPLOADS, else the temp folder). Next: put the file there or set WEBSCOPE_UPLOADS", path)
	}
	base := filepath.Base(real)
	if strings.HasPrefix(base, ".") {
		return nil, fmt.Errorf("%s is a hidden file; upload does not read those", base)
	}
	if i, err := os.Stat(real); err != nil || !i.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", base)
	} else if i.Size() > maxUploadBytes {
		return nil, fmt.Errorf("%s is %d bytes; the limit is %d", base, i.Size(), maxUploadBytes)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return nil, err
	}
	typ := mime.TypeByExtension(strings.ToLower(filepath.Ext(base)))
	if typ == "" {
		typ = http.DetectContentType(data)
	}
	return &upload{name: base, typ: typ, data: data}, nil
}

// prepareUpload keeps the file the call names under a random id, for the helper to fetch, and returns the arguments of the snippet.
func (p *Tool) prepareUpload(args map[string]any) (map[string]any, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("upload needs path (the file to attach) and field, ref or css (the file input; not needed when the page has only one)")
	}
	u, err := readUpload(path)
	if err != nil {
		return nil, err
	}
	b := make([]byte, 8)
	rand.Read(b)
	id := hex.EncodeToString(b)
	p.mu.Lock()
	if p.uploads == nil {
		p.uploads = map[string]*upload{}
	}
	for k := range p.uploads {
		if len(p.uploads) < 5 {
			break
		}
		delete(p.uploads, k) // files prepared and never fetched do not pile up
	}
	p.uploads[id] = u
	p.mu.Unlock()
	out := map[string]any{"id": id, "name": u.name, "type": u.typ, "size": len(u.data)}
	for _, k := range []string{"field", "ref", "css"} {
		if v, ok := args[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

// serveUpload gives the helper the bytes of a prepared file, once.
func (p *Tool) serveUpload(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	p.mu.Lock()
	u := p.uploads[id]
	delete(p.uploads, id)
	p.mu.Unlock()
	if u == nil {
		http.Error(w, "no such file", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(u.data)
}
