package data

import (
	"fmt"

	"github.com/go-pg/migrations"
	"github.com/go-pg/pg"

	"github.com/dapperlabs/bamboo-node/internal/nodes/access/config"
)

// DAL Data Access Layer
type DAL struct {
	db *pg.DB
}

// New ..
func New(c *config.Config) *DAL {

	options := &pg.Options{
		Addr:     c.PgAddr,
		User:     c.PgUser,
		Password: c.PgPassword,
		Database: c.PgDatabase,
	}

	db := pg.Connect(options)

	return &DAL{
		db: db,
	}
}

// MigrateUp ..
func (d *DAL) MigrateUp() error {
	return d.migrate("up")
}

// MigrateDown ..
func (d *DAL) MigrateDown() error {
	return d.migrate("reset")
}

func (d *DAL) migrate(cmd string) error {

	// Migrations
	migrations.DefaultCollection.DiscoverSQLMigrations("migrations/")
	_, _, _ = migrations.Run(d.db, "init")
	oldVersion, newVersion, err := migrations.Run(d.db, cmd)
	if err != nil {
		return err
	}
	if newVersion != oldVersion {
		fmt.Printf("Migration %v: from version %d to %d\n", cmd, oldVersion, newVersion)
	} else {
		fmt.Printf("Migration %v: not needed. version is %d\n", cmd, oldVersion)
	}

	return nil

}
