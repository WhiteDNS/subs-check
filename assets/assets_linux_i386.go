//go:build linux && 386
// +build linux,386

package assets

import (
	_ "embed"
)

// node does not support the linux 386 architecture.
var EmbeddedNode []byte
