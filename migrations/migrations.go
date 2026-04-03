// Package migrations embeds the database migration SQL files.
package migrations

import "embed"

// FS contains all migration SQL files.
//
//go:embed *.sql
var FS embed.FS
