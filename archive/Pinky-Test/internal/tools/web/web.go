// Package web provides web-related tools for Pinky.
//
// Available tools:
//   - web_fetch: Fetch content from a URL (GET, POST, etc.)
//   - web_parse_html: Parse HTML and extract data with CSS selectors
//   - web_download: Download files from URLs to the filesystem
//
// All web tools are marked as low risk in the permission system.
package web

import (
	"time"

	"github.com/normanking/pinky/internal/tools"
)

// RegisterAll registers all web tools with the given registry.
func RegisterAll(registry *tools.Registry) {
	registry.Register(NewFetchTool())
	registry.Register(NewParseHTMLTool())
	registry.Register(NewDownloadTool())
}

// RegisterWithOptions registers web tools with custom options.
func RegisterWithOptions(registry *tools.Registry, opts WebToolsOptions) {
	// Register FetchTool with options
	fetchOpts := []FetchOption{}
	if opts.FetchTimeout > 0 {
		fetchOpts = append(fetchOpts, WithTimeout(opts.FetchTimeout))
	}
	if opts.MaxFetchSize > 0 {
		fetchOpts = append(fetchOpts, WithMaxBodySize(opts.MaxFetchSize))
	}
	if opts.UserAgent != "" {
		fetchOpts = append(fetchOpts, WithUserAgent(opts.UserAgent))
	}
	registry.Register(NewFetchTool(fetchOpts...))

	// Register ParseHTMLTool (no options currently)
	registry.Register(NewParseHTMLTool())

	// Register DownloadTool with options
	downloadOpts := []DownloadOption{}
	if opts.DownloadTimeout > 0 {
		downloadOpts = append(downloadOpts, WithDownloadTimeout(opts.DownloadTimeout))
	}
	if opts.MaxDownloadSize > 0 {
		downloadOpts = append(downloadOpts, WithMaxFileSize(opts.MaxDownloadSize))
	}
	if len(opts.AllowedDownloadDirs) > 0 {
		downloadOpts = append(downloadOpts, WithAllowedDirs(opts.AllowedDownloadDirs))
	}
	registry.Register(NewDownloadTool(downloadOpts...))
}

// WebToolsOptions configures the web tools.
type WebToolsOptions struct {
	// FetchTool options
	FetchTimeout time.Duration
	MaxFetchSize int64
	UserAgent    string

	// DownloadTool options
	DownloadTimeout     time.Duration
	MaxDownloadSize     int64
	AllowedDownloadDirs []string
}
