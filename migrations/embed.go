package migrations

import "embed"

// Files contains all SQL migrations embedded in the binary.
//
//go:embed *.sql
var Files embed.FS
