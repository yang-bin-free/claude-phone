// Package web contains the desktop and mobile web assets.
package web

import "embed"

// Assets contains the shared chat shell and desktop administration assets.
//
//go:embed chat admin
var Assets embed.FS
