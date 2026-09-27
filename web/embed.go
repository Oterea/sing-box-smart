// Package web embeds browser assets so the program can run as a single binary.
package web

import "embed"

//go:embed index.html styles.css app.js
var Files embed.FS
