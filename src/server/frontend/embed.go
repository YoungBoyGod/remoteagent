package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the frontend dist filesystem.
// Returns nil if dist is empty (dev mode without frontend build).
func DistFS() fs.FS {
	entries, err := fs.ReadDir(distFS, "dist")
	if err != nil || len(entries) == 0 || (len(entries) == 1 && entries[0].Name() == ".gitkeep") {
		return nil
	}
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	return sub
}
