// Package assets embeds only locally reconstructed original game resources.
// Run go run ./cmd/extract before building; no original data belongs in Git.
package assets

import "embed"

// Files contains the reproducible manifest, decoded graphics, and original tables.
// The disk's executable wrapper and crack introduction are excluded.
//
//go:embed manifest.json graphics/*.png unpacked/*.bin
var Files embed.FS
