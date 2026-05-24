// Package migrations contains the *.sql migration files.
package migrations

import (
	"embed"

	migrate "github.com/rubenv/sql-migrate"
)

//go:embed *.sql
var FS embed.FS

func GetMigrationSource() *migrate.EmbedFileSystemMigrationSource {
	return &migrate.EmbedFileSystemMigrationSource{
		FileSystem: FS,
		Root:       ".",
	}
}
