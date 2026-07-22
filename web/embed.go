package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:ui/dist
var distFS embed.FS

// spaHandler serves the embedded Vue build. Unknown non-API paths fall back to
// index.html (client-side routing). When the frontend has not been built yet
// (only .gitkeep present), it serves a plain notice instead of 404.
func (s *server) spaHandler() http.Handler {
	sub, err := fs.Sub(distFS, "ui/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(sub, "index.html"); err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(200)
			_, _ = w.Write([]byte("Cartographer web UI not built. Run: cd web/ui && npm install && npm run build"))
			return
		}
		// serve the file if it exists; otherwise fall back to index.html
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
