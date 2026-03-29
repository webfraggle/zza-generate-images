// Package web embeds the frontend HTML templates and static assets.
package web

import "embed"

//go:embed templates static
var FS embed.FS
