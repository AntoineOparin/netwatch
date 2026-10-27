// Package webui embeds the built React Router SPA into the hub binary, so the
// hub ships as a single file. `make web` writes the build into build/client.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:build
var build embed.FS

// Handler serves the SPA. Paths without a file extension that don't match a
// file fall back to index.html, so client-side routes like /devices survive a
// page reload. Missing assets (/assets/x.js) still 404.
func Handler() http.Handler {
	client, err := fs.Sub(build, "build/client")
	if err == nil {
		_, err = fs.Stat(client, "index.html")
	}
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "dashboard not built: run `make web` and rebuild the hub", http.StatusNotFound)
		})
	}

	files := http.FileServerFS(client)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if _, err := fs.Stat(client, name); name == "" || err == nil || path.Ext(name) != "" {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFileFS(w, r, client, "index.html")
	})
}
