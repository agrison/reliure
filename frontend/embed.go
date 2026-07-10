// Package frontend embeds the built Svelte assets so the desktop binary is
// fully self-contained. It intentionally holds no logic: keeping the embed in
// the same directory as the assets it references lets the app entry point
// (cmd/app) live elsewhere without go:embed's "no parent path" restriction.
package frontend

import "embed"

// Assets is the compiled frontend (the Vite output in frontend/dist). The Wails
// asset server locates index.html within it automatically (via fs.Sub), so the
// embed prefix does not need stripping.
//
//go:embed all:dist
var Assets embed.FS
