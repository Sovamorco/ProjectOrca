package migrations

import (
	"embed"

	"github.com/sovamorco/errorx"
	"github.com/uptrace/bun/migrate"
)

//go:embed migrations
var migrationsFS embed.FS

func NewMigrations() (*migrate.Migrations, error) {
	migrations := migrate.NewMigrations()
	if err := migrations.Discover(migrationsFS); err != nil {
		return nil, errorx.Decorate(err, "discover migrations")
	}

	return migrations, nil
}
