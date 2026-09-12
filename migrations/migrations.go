package migrations

import "embed"

//go:embed mysql/*.sql
var Files embed.FS
