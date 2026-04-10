package static

import (
	"embed"
	"io/fs"
)

// Include underscore-prefixed export paths such as _next and _not-found.
//
//go:embed all:out all:out/**
var embeddedFiles embed.FS

// StaticFS is nil in source checkouts until frontend assets are bundled into the binary by the release pipeline.
var StaticFS fs.FS

func init() {
	sub, err := fs.Sub(embeddedFiles, "out")
	if err != nil {
		return
	}
	StaticFS = sub
}
