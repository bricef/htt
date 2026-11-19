package gui

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// web is the page and its assets, served as they are: no build step.
// web/vendor holds the third-party ones, with their licences and a
// README saying where each came from.
//
//go:embed web
var web embed.FS

// contentTypes are the types of the files in web, by extension; any
// other file is plain text (the licences, the README).
var contentTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".woff2": "font/woff2",
}

// assets serves the files in web by their paths, index.html for "/".
// A directory, or anything else that isn't a file, is a 404.
func assets() http.Handler {
	root, err := fs.Sub(web, "web")
	if err != nil {
		panic(err) // the embedded directory is always there
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		info, err := fs.Stat(root, name) // an invalid path ("..", "a//b") is an error too
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		body, err := fs.ReadFile(root, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		contentType, ok := contentTypes[path.Ext(name)]
		if !ok {
			contentType = "text/plain; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
	})
}
