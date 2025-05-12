package cmd

import (
	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
)

// InitStorages initializes the badger storages
func InitStorages() (*storage.All, *badger.DB) {
	db := common.InitStorage(flagDatadir)
	storages := common.InitStorages(db)
	return storages, db
}

// WithStorage runs the given function with the storage dependending on the flags
// only one flag (datadir / pebble-dir) is allowed to be set
func WithStorage(f func(storage.DB) error) error {
	return common.WithStorage(
		common.DBDirs{
			Datadir:   flagDatadir,
			Pebbledir: flagPebbleDir,
		}, f)
}

// InitBadgerAndPebble initializes the badger and pebble storages
func InitBadgerAndPebble() (bdb *badger.DB, pdb *pebble.DB, err error) {
	return common.InitBadgerAndPebble(common.DBDirs{
		Datadir:   flagDatadir,
		Pebbledir: flagPebbleDir,
	})
}

// WithBadgerAndPebble runs the given function with the badger and pebble storages
// it ensures that the storages are closed after the function is done
func WithBadgerAndPebble(f func(*badger.DB, *pebble.DB) error) error {
	return common.WithBadgerAndPebble(
		common.DBDirs{
			Datadir:   flagDatadir,
			Pebbledir: flagPebbleDir,
		}, f)
}
