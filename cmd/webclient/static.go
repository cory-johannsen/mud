package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
)

//go:embed ui/dist
var embeddedUI embed.FS

// buildStaticHandler constructs an http.Handler for static files.
// If staticDir is non-empty, files are served from the filesystem (dev mode).
// Otherwise the embedded ui/dist is used.
//
// Postcondition: Returns a handler that serves index.html for all unmatched paths.
func buildStaticHandler(staticDir string) http.Handler {
	var fsys fs.FS
	if staticDir != "" {
		fsys = os.DirFS(staticDir)
	} else {
		sub, err := fs.Sub(embeddedUI, "ui/dist")
		if err != nil {
			panic("embedded ui/dist missing: " + err.Error())
		}
		fsys = sub
	}
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}
		f, err := fsys.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for all non-file paths.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
