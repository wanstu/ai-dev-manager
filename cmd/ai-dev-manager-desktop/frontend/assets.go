package frontend

import (
	"embed"
	"io/fs"
)

// SharedAssets is the single source used by both Wails Desktop and Gateway Web.
//
//go:embed *.html *.css *.js assets
var sharedAssets embed.FS

func Assets() (fs.FS, error) {
	return fs.Sub(sharedAssets, ".")
}
