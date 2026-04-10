// Package dashboardfs embeds the compiled React dashboard and exposes it as an
// http.FileSystem so the router can serve it at /_/*.
package dashboardfs

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/index.html dist/assets
var dashboardFiles embed.FS

// HTTPRoot returns an http.FileSystem rooted at the embedded dist directory.
// The dist directory is produced by running `npm run build` inside ./dashboard.
func HTTPRoot() http.FileSystem {
	sub, err := fs.Sub(dashboardFiles, "dist")
	if err != nil {
		panic("dashboardfs: could not sub into dist: " + err.Error())
	}
	return http.FS(sub)
}
