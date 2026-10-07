// Package web holds the built interface and the handler that serves it.
package web

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	assetsPrefix   = "/assets/"
	immutableCache = "public, max-age=31536000, immutable"
	revalidate     = "no-cache"
	indexName      = "index.html"
)

// contentTypes maps the extension of every file the build may hold to its
// media type.
var contentTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".woff2": "font/woff2",
	".svg":   "image/svg+xml",
	".ico":   "image/x-icon",
	".json":  "application/json",
}

// Handler serves the files of the built interface: a file under /assets/
// with a year-long immutable cache, any other file with no-cache, a missing
// file under /assets/ as a plain 404, and index.html for any other path.
func Handler() http.Handler {
	dist, err := fs.Sub(build, "dist")
	if err != nil {
		panic("web: cannot open the embedded build: " + err.Error())
	}
	return handler(dist)
}

func handler(files fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" || !isFile(files, name) {
			if strings.HasPrefix("/"+name, assetsPrefix) {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("Not Found"))
				return
			}
			name = indexName
		}
		body, err := fs.ReadFile(files, name)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		cache := revalidate
		if strings.HasPrefix("/"+name, assetsPrefix) {
			cache = immutableCache
		}
		h := w.Header()
		h.Set("Cache-Control", cache)
		if ctype, ok := contentTypes[path.Ext(name)]; ok {
			h.Set("Content-Type", ctype)
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
	})
}

// isFile reports whether name is a regular file of files.
func isFile(files fs.FS, name string) bool {
	info, err := fs.Stat(files, name)
	return err == nil && !info.IsDir()
}
