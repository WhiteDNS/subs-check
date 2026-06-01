//go:build !darwin && !linux && !windows
// +build !darwin,!linux,!windows

package assets

import (
	_ "embed"
)

// Other unsupported platforms.
// NODEBIN_PATH must be specified manually.
var EmbeddedNode []byte
