package keyvalue

import (
	"fmt"

	"github.com/go-pg/migrations"
	"github.com/go-pg/pg"
)

// PostgresDB ..
type PostgresDB struct {
	db *pg.DB
}

// NewPostgresDB ..
func NewPostgresDB(addr, user, password, dbname string) *PostgresDB {

	options := &pg.Options{
		Addr:     addr,
		User:     user,
		Password: password,
		Database: dbname,
	}

	db := pg.Connect(options)

	return &PostgresDB{
		db: db,
	}
}

// NewQuery ..
func (d *PostgresDB) NewQuery() QueryBuilder {
	return &pgSQLQuery{db: d.db}
}

// MigrateUp ..
func (d *PostgresDB) MigrateUp() error {
	return d.migrate("up")
}

// MigrateDown ..
func (d *PostgresDB) MigrateDown() error {
	return d.migrate("reset")
}

func (d *PostgresDB) migrate(cmd string) error {

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
