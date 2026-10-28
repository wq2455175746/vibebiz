package migrations

import "embed"

// FS contains the versioned SQL migrations applied by cmd/migrate.
//
//go:embed *.sql
var FS embed.FS
