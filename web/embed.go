// Package web embeds browser assets so the program can run as a single binary.
package web

import "embed"

//go:embed index.html styles.css responsive.css app.js stability-detail.js stability-view.js settings.js settings-dialog.js logo.svg node-list.js
var Files embed.FS
