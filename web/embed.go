package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

// StaticHandler returns an http.Handler that serves the embedded dashboard files.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to get static sub-filesystem: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
