package migrations

import (
	"embed"
	"github.com/uptrace/bun/migrate"
)

var (
	//go:embed migrations
	migrationsFS embed.FS
	Migrations   = migrate.NewMigrations()
)

func init() {
	if err := Migrations.Discover(migrationsFS); err != nil {
		panic(err)
	}
}
