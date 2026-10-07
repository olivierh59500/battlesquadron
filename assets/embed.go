// Package assets embeds locally reconstructed original resources and demo inputs.
// Run make assets before building; generated data never belongs in Git.
package assets

import "embed"

// Files contains the manifest, original graphics/tables, and generated joystick
// commands. The disk's executable wrapper and crack introduction are excluded.
//
//go:embed manifest.json graphics/*.png unpacked/*.bin expert.bsinput
var Files embed.FS
