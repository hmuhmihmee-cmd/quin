package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/*
var assetFiles embed.FS

func assetsHandler() http.Handler {
	assets, err := fs.Sub(assetFiles, "assets")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/assets/", http.FileServer(http.FS(assets)))
}
