// Package webui serves the web UI built from web/ into dist/ by `make web`.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var files embed.FS

// Handler serves dist/, or a short plain-text page when the UI is not built.
func Handler() http.Handler {
	dist, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err) // embedded at build time
	}
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("web UI not built; run make web\n"))
		})
	}
	return http.FileServerFS(dist)
}
