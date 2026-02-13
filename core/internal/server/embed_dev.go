//go:build !embed_prism
// +build !embed_prism

package server

// Default mode: No embedded assets.
// The server will serve a placeholder page and expect
// the React app to be run separately via Vite.
//
// Build with embedded assets (production):
//   cd prism && npm run build
//   go build -tags embed_prism ./cmd/cortex
//
// Build without embedded assets (development):
//   go build ./cmd/cortex
//   Then run Vite: cd prism && npm run dev
