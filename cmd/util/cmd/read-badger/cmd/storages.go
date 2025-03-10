package cmd

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

// InitStorages initializes the badger storages
func InitStorages() (*storage.All, *badger.DB) {
	db := common.InitStorage(flagDatadir)
	storages := common.InitStorages(db)
	return storages, db
}

// WithStorage runs the given function with the storage dependending on the flags
// only one flag (datadir / pebbledir) is allowed to be set
func WithStorage(f func(storage.DB) error) error {
	if flagPebbleDir != "" {
		if flagDatadir != "" {
			log.Warn().Msg("both --datadir and --pebbledir are set, using --pebbledir")
		}

		db, err := pebblestorage.MustOpenDefaultPebbleDB(log.Logger, flagPebbleDir)
		if err != nil {
			return err
		}

		defer db.Close()
		return f(pebbleimpl.ToDB(db))
	}

	if flagDatadir != "" {
		db := common.InitStorage(flagDatadir)
		defer db.Close()
		return f(badgerimpl.ToDB(db))
	}

	return fmt.Errorf("must specify either --datadir or --pebbledir")
}

// InitBadgerAndPebble initializes the badger and pebble storages
func InitBadgerAndPebble() (bdb *badger.DB, pdb *pebble.DB, err error) {
	if flagDatadir == "" {
		return nil, nil, fmt.Errorf("must specify --datadir")
	}

	if flagPebbleDir == "" {
		return nil, nil, fmt.Errorf("must specify --pebbledir")
	}

	pdb, err = pebblestorage.MustOpenDefaultPebbleDB(
		log.Logger.With().Str("pebbledb", "protocol").Logger(), flagPebbleDir)
	if err != nil {
		return nil, nil, err
	}

	bdb = common.InitStorage(flagDatadir)

	return bdb, pdb, nil
}

// WithBadgerAndPebble runs the given function with the badger and pebble storages
// it ensures that the storages are closed after the function is done
func WithBadgerAndPebble(f func(*badger.DB, *pebble.DB) error) error {
	bdb, pdb, err := InitBadgerAndPebble()
	if err != nil {
		return err
	}

	defer bdb.Close()
	defer pdb.Close()

	return f(bdb, pdb)
}
