package common

import (
	"fmt"

	"github.com/cockroachdb/pebble/v2"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

// DBDirs is a struct that holds the datadir (location of badger database) and pebbledir (location of pebble database) .
// This struct helps to prevent mistakes, such confusing pebbledir and datadir.
type DBDirs struct {
	Datadir   string
	Pebbledir string
}

type UsedDB int

const (
	UsedDBBadger UsedDB = iota
	UsedDBPebble
)

func (usedDB UsedDB) String() string {
	switch usedDB {
	case UsedDBBadger:
		return "badger"
	case UsedDBPebble:
		return "pebble"
	default:
		return "unknown"
	}
}

// DBDirs is a struct that holds the used datadir and pebble-dir
// this struct help prevents mistakes from passing the wrong dir, such as pass pebbledir as datadir
type TwoDBDirs struct {
	BadgerDir string
	PebbleDir string
}

// OneDBDir is a struct that holds the used database dir
type OneDBDir struct {
	UseDB UsedDB
	DBDir string
}

// ParseOneDBUsedDir validates the database flags and
// return a single database dir depending on the UseDB flag, which determine the database type
func ParseOneDBUsedDir(flags DBFlags) (OneDBDir, error) {
	if flags.UseDB == "pebble" {
		if flags.PebbleDir == "" {
			return OneDBDir{}, fmt.Errorf("--pebble-dir is required when using pebble db")
		}
		return OneDBDir{
			UseDB: UsedDBPebble,
			DBDir: flags.PebbleDir,
		}, nil
	}

	if flags.UseDB == "badger" {
		if flags.BadgerDir == "" {
			return OneDBDir{}, fmt.Errorf("--datadir is required when using badger db")
		}
		return OneDBDir{
			UseDB: UsedDBBadger,
			DBDir: flags.BadgerDir,
		}, nil
	}

	return OneDBDir{}, fmt.Errorf("unknown database type: %s", flags.UseDB)
}

// ParseTwoDBDirs requires both badger and pebble dirs are defined in flags
func ParseTwoDBDirs(flags DBFlags) (TwoDBDirs, error) {
	if flags.BadgerDir == "" {
		return TwoDBDirs{}, fmt.Errorf("--datadir is required when using badger db")
	}

	if flags.PebbleDir == "" {
		return TwoDBDirs{}, fmt.Errorf("--pebble-dir is required when using pebble db")
	}

	return TwoDBDirs{
		BadgerDir: flags.BadgerDir,
		PebbleDir: flags.PebbleDir,
	}, nil
}

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
	if ok {
		db, err := pebblestorage.ShouldOpenDefaultPebbleDB(log.Logger, datadir)
		if err != nil {
			return nil, fmt.Errorf("could not open pebble db at %s: %w", datadir, err)
		}
		log.Info().Msgf("using pebble db at %s", datadir)
		return pebbleimpl.ToDB(db), nil
	}

	return nil, fmt.Errorf("could not determine storage type (not badger, nor pebble) for directory %s", datadir)
}

func InitBadgerDBStorage(datadir string) *badger.DB {
	return InitStorageWithTruncate(datadir, false)
}

func InitStorageWithTruncate(datadir string, truncate bool) *badger.DB {
	opts := badger.
		DefaultOptions(datadir).
		WithKeepL0InMemory(true).
		WithLogger(nil).
		WithTruncate(truncate)

	db, err := storagebadger.SafeOpen(opts)
	if err != nil {
		log.Fatal().Err(err).Msg("could not open key-value store")
	}

	// in order to void long iterations with big keys when initializing with an
	// already populated database, we bootstrap the initial maximum key size
	// upon starting
	err = operation.RetryOnConflict(db.Update, func(tx *badger.Txn) error {
		return operation.InitMax(tx)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize max tracker")
	}

	return db
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

func InitStorages(db storage.DB) *store.All {
	metrics := &metrics.NoopCollector{}
	return store.InitAll(metrics, db)
}

func InitBadgerStorages(db *badger.DB) *storage.All {
	metrics := &metrics.NoopCollector{}

	return storagebadger.InitAll(metrics, db)
}

// WithStorage runs the given function with the storage depending on the flags.
func WithStorage(flags DBFlags, f func(storage.DB) error) error {
	usedDir, err := ParseOneDBUsedDir(flags)
	if err != nil {
		return fmt.Errorf("could not parse db flags: %w", err)
	}

	log.Info().Msgf("using %s db at %s", usedDir.UseDB, usedDir.DBDir)

	if usedDir.UseDB == UsedDBPebble {
		db, err := pebblestorage.ShouldOpenDefaultPebbleDB(log.Logger, usedDir.DBDir)
		if err != nil {
			return fmt.Errorf("can not open pebble db at %v: %w", usedDir.DBDir, err)
		}

		defer db.Close()

		return f(pebbleimpl.ToDB(db))
	}

	if usedDir.UseDB == UsedDBBadger {
		return fmt.Errorf("badger db is no longer supported for protocol data, please use pebble db")
	}

	return fmt.Errorf("unexpected error")
}

// InitBadgerAndPebble initializes the badger and pebble storages
func InitBadgerAndPebble(dirs TwoDBDirs) (bdb *badger.DB, pdb *pebble.DB, err error) {
	pdb, err = pebblestorage.ShouldOpenDefaultPebbleDB(
		log.Logger.With().Str("pebbledb", "protocol").Logger(), dirs.PebbleDir)
	if err != nil {
		return nil, nil, err
	}

	bdb = InitBadgerDBStorage(dirs.BadgerDir)

	return bdb, pdb, nil
}
