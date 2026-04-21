package templates

import "embed"

// FS contains the embedded admin HTML templates for the single-service MVP.
//
//go:embed *.html
var FS embed.FS
