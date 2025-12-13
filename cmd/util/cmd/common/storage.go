package common

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

func InitStorage(datadir string) (storage.DB, error) {
	ok, err := IsBadgerFolder(datadir)
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, fmt.Errorf("badger db is no longer supported for protocol data, datadir: %v", datadir)
	}

	ok, err = IsPebbleFolder(datadir)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("could not determine storage type (not badger, nor pebble) for directory %s", datadir)
	}

	db, err := pebblestorage.ShouldOpenDefaultPebbleDB(log.Logger, datadir)
	if err != nil {
		return nil, fmt.Errorf("could not open pebble db at %s: %w", datadir, err)
	}
	log.Info().Msgf("using pebble db at %s", datadir)
	return pebbleimpl.ToDB(db), nil

}

// IsBadgerFolder checks if the given directory is a badger folder.
// It returns error if the folder is empty or not exists.
// it returns error if the folder is not empty, but misses some required badger files.
func IsBadgerFolder(dataDir string) (bool, error) {
	return storagebadger.IsBadgerFolder(dataDir)
}

// IsPebbleFolder checks if the given directory is a pebble folder.
// It returns error if the folder is empty or not exists.
// it returns error if the folder is not empty, but misses some required pebble files.
func IsPebbleFolder(dataDir string) (bool, error) {
	return pebblestorage.IsPebbleFolder(dataDir)
}

func InitStorages(db storage.DB, chainID flow.ChainID) *store.All {
	metrics := &metrics.NoopCollector{}
	return store.InitAll(metrics, db, chainID)
}

// WithStorage runs the given function with the storage depending on the flags.
func WithStorage(datadir string, f func(storage.DB) error) error {
	log.Info().Msgf("opening protocol db at datadir: %v", datadir)

	ok, err := IsPebbleFolder(datadir)
	if err != nil {
		return fmt.Errorf("fail to check if folder stores pebble data: %w", err)
	}

	if !ok {
		ok, err := IsBadgerFolder(datadir)
		if err != nil {
			return fmt.Errorf("fail to check if folder stores badger data: %w", err)
		}
		if ok {
			return fmt.Errorf("badger db is no longer supported for protocol data, datadir: %v", datadir)
		}

		return fmt.Errorf("the given datadir does not contain a valid pebble db: %v", datadir)
	}

	db, err := pebblestorage.ShouldOpenDefaultPebbleDB(log.Logger, datadir)
	if err != nil {
		return fmt.Errorf("can not open pebble db at %v: %w", datadir, err)
	}

	defer db.Close()

	log.Info().Msgf("opened pebble protocol database")

	return f(pebbleimpl.ToDB(db))
}
