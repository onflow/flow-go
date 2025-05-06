package cmd

import (
	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
)

// InitStorages initializes the badger storages
func InitStorages() (*storage.All, *badger.DB) {
	flagDBs := common.ReadDBFlags()
	usedDir, err := common.ParseOneDBUsedDir(flagDBs)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse db flag")
	}

	db := common.InitStorage(usedDir.DBDir)
	storages := common.InitStorages(db)
	return storages, db
}

// WithStorage runs the given function with the storage dependending on the flags
// only one flag (datadir / pebble-dir) is allowed to be set
func WithStorage(f func(storage.DB) error) error {
	flagDBs := common.ReadDBFlags()
	return common.WithStorage(flagDBs, f)
}

// InitBadgerAndPebble initializes the badger and pebble storages
func InitBadgerAndPebble() (bdb *badger.DB, pdb *pebble.DB, err error) {
	flagDBs := common.ReadDBFlags()
	dbDirs, err := common.ParseTwoDBDirs(flagDBs)
	if err != nil {
		return nil, nil, err
	}
	return common.InitBadgerAndPebble(dbDirs)
}

// WithBadgerAndPebble runs the given function with the badger and pebble storages
// it ensures that the storages are closed after the function is done
func WithBadgerAndPebble(f func(*badger.DB, *pebble.DB) error) error {
	flagDBs := common.ReadDBFlags()
	return common.WithBadgerAndPebble(flagDBs, f)
}
