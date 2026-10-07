package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:web
var frontendAssets embed.FS

func embeddedFrontend() http.Handler {
	assets, err := fs.Sub(frontendAssets, "web")
	if err != nil {
		panic(err)
	}
	return staticFrontend(assets)
}
func staticFrontend(assets fs.FS) http.Handler {
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/s/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		for _, part := range strings.Split(name, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		info, err := fs.Stat(assets, name)
		if (err != nil || info.IsDir()) && path.Ext(name) == "" {
			if page, pageErr := fs.Stat(assets, name+".html"); pageErr == nil && !page.IsDir() {
				name += ".html"
				info, err = page, nil
				r = r.Clone(r.Context())
				r.URL.Path = "/" + name
			}
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			if _, err = fs.Stat(assets, path.Join(name, "index.html")); err != nil {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if strings.HasPrefix(name, "_next/static/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
