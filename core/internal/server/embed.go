//go:build embed_prism
// +build embed_prism

package server

import "embed"

// Embed the React SPA build output.
// This directive embeds the prism/dist directory into the binary.
// The build process is:
//   1. cd prism && npm run build
//   2. go build -tags embed_prism ./cmd/cortex
//
// To run without embedded assets (dev mode), build normally:
//   go build ./cmd/cortex

//go:embed all:prism/dist
var embeddedAssets embed.FS

func init() {
	Assets = embeddedAssets
}
