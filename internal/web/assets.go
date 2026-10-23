package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"strings"
)

//go:embed all:static
var staticFS embed.FS

type manifestEntry struct {
	File string   `json:"file"`
	CSS  []string `json:"css"`
}

var assetManifest map[string]manifestEntry

func loadManifest() map[string]manifestEntry {
	if assetManifest != nil {
		return assetManifest
	}
	assetManifest = map[string]manifestEntry{}
	data, err := fs.ReadFile(staticFS, "static/.vite/manifest.json")
	if err != nil {
		return assetManifest
	}
	_ = json.Unmarshal(data, &assetManifest)
	return assetManifest
}

// Asset returns the public URL for a Vite entry or source file.
func Asset(name string) string {
	if e, ok := loadManifest()[name]; ok && e.File != "" {
		return "/static/" + strings.TrimPrefix(e.File, "/")
	}
	if name == "src/main.ts" {
		return "/static/main.js"
	}
	return "/static/" + path.Base(name)
}

// AssetCSS returns compiled stylesheet URLs associated with an entry.
func AssetCSS(name string) []string {
	if e, ok := loadManifest()[name]; ok {
		out := make([]string, 0, len(e.CSS))
		for _, css := range e.CSS {
			out = append(out, "/static/"+strings.TrimPrefix(css, "/"))
		}
		return out
	}
	return []string{"/static/app.css"}
}

// StaticFS exposes built assets for the HTTP file server.
func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return staticFS
	}
	return sub
}
