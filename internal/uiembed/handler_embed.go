//go:build uiembed

package uiembed

import (
	"bytes"
	"embed"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

var (
	//go:embed dist/**
	embeddedDist embed.FS

	errNoBasePath = errors.New("base path is required")
)

func Available() bool {
	return true
}

func NewHandler(basePath string) (http.Handler, error) {
	if strings.TrimSpace(basePath) == "" {
		return nil, errNoBasePath
	}

	dist, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cleanPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		if cleanPath == "/" {
			cleanPath = "/index.html"
		}
		name := strings.TrimPrefix(cleanPath, "/")
		if name == "" {
			name = "index.html"
		}

		if fileExists(dist, name) {
			serveFile(dist, w, r, name)
			return
		}

		if path.Ext(cleanPath) != "" {
			http.NotFound(w, r)
			return
		}

		serveFile(dist, w, r, "index.html")
	}), nil
}

func fileExists(dist fs.FS, name string) bool {
	if name == "" || name == "." {
		name = "index.html"
	}
	file, err := dist.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func serveFile(dist fs.FS, w http.ResponseWriter, r *http.Request, name string) {
	file, err := dist.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read asset", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, name, info.ModTime(), bytes.NewReader(data))
}
