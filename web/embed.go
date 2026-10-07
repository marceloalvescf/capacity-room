package web

import "embed"

// build holds the built interface, the output of npm run build.
//
//go:embed all:dist
var build embed.FS
