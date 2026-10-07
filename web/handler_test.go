package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"testing/fstest"
)

const (
	indexBody = "<html><div id=\"root\"></div></html>"
	assetBody = "console.log(1)"
	bootBody  = "boot()"
	immutable = "public, max-age=31536000, immutable"
	jsType    = "text/javascript; charset=utf-8"
	htmlType  = "text/html; charset=utf-8"
	plainType = "text/plain; charset=utf-8"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":      {Data: []byte(indexBody)},
		"assets/app-1.js": {Data: []byte(assetBody)},
		"theme-boot.js":   {Data: []byte(bootBody)},
	}
}

func serve(t *testing.T, method, target string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "/", nil)
	r.URL.Path = target
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	handler(testFS()).ServeHTTP(w, r)
	return w
}

func TestStaticFallback(t *testing.T) {
	cases := []struct {
		name, method, target string
		status               int
		cache, ctype, body   string
	}{
		{"root", "GET", "/", 200, "no-cache", htmlType, indexBody},
		{"client route", "GET", "/account", 200, "no-cache", htmlType, indexBody},
		{"unknown page", "GET", "/no-such-page", 200, "no-cache", htmlType, indexBody},
		{"asset", "GET", "/assets/app-1.js", 200, immutable, jsType, assetBody},
		{"missing asset", "GET", "/assets/missing.js", 404, "", plainType, "Not Found"},
		{"double slash asset", "GET", "//assets/missing.js", 404, "", plainType, "Not Found"},
		{"dot dot out of assets", "GET", "/assets/../nothing", 200, "no-cache", htmlType, indexBody},
		{"root file", "GET", "/theme-boot.js", 200, "no-cache", jsType, bootBody},
		{"head", "HEAD", "/account", 200, "no-cache", htmlType, ""},
		{"escape", "GET", "/../etc/passwd", 200, "no-cache", htmlType, indexBody},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := serve(t, c.method, c.target, nil)
			if w.Code != c.status {
				t.Fatalf("status %d, want %d", w.Code, c.status)
			}
			if got := w.Header().Get("Cache-Control"); got != c.cache {
				t.Errorf("Cache-Control %q, want %q", got, c.cache)
			}
			if got := w.Header().Get("Content-Type"); got != c.ctype {
				t.Errorf("Content-Type %q, want %q", got, c.ctype)
			}
			if got := w.Body.String(); got != c.body {
				t.Errorf("body %q, want %q", got, c.body)
			}
		})
	}
}

func TestStaticTypes(t *testing.T) {
	want := map[string]string{
		".html":  htmlType,
		".js":    jsType,
		".css":   "text/css; charset=utf-8",
		".woff2": "font/woff2",
		".svg":   "image/svg+xml",
		".ico":   "image/x-icon",
		".json":  "application/json",
	}
	if len(contentTypes) != len(want) {
		t.Errorf("table holds %d types, want %d", len(contentTypes), len(want))
	}
	for ext, ctype := range want {
		files := fstest.MapFS{"index.html": {Data: []byte("i")}, "file" + ext: {Data: []byte("x")}}
		w := httptest.NewRecorder()
		handler(files).ServeHTTP(w, httptest.NewRequest("GET", "/file"+ext, nil))
		if got := w.Header().Get("Content-Type"); got != ctype {
			t.Errorf("%s: Content-Type %q, want %q", ext, got, ctype)
		}
	}
}

func TestStaticConditional(t *testing.T) {
	w := serve(t, "GET", "/assets/app-1.js", map[string]string{"Range": "bytes=0-3"})
	if w.Code != http.StatusPartialContent {
		t.Fatalf("status %d, want 206", w.Code)
	}
	if got := w.Body.String(); got != assetBody[:4] {
		t.Fatalf("body %q, want %q", got, assetBody[:4])
	}
}

func TestBuildHoldsOnlyKnownTypes(t *testing.T) {
	dist, err := fs.Sub(build, "dist")
	if err != nil {
		t.Fatal(err)
	}
	script := false
	err = fs.WalkDir(dist, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := path.Ext(p)
		if _, ok := contentTypes[ext]; !ok {
			t.Errorf("%s has no entry in the table of types", p)
		}
		if ext == ".js" && strings.HasPrefix(p, "assets/") {
			script = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		t.Error(err)
	}
	if !script {
		t.Error("the build holds no script under assets/")
	}
}

func readIndex(t *testing.T) string {
	t.Helper()
	b, err := build.ReadFile("dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBuildHasNoInlineStyle(t *testing.T) {
	index := readIndex(t)
	if strings.Contains(index, "style=") || strings.Contains(index, "<style") {
		t.Fatal("index.html holds inline style")
	}
}
