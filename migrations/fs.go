package migrations

import "embed"

// FS embeds all SQL migrations in this directory.
//
//go:embed *.sql
var FS embed.FS
