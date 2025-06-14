package dbtest

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// helper types and functions
type WithWriter func(func(storage.Writer) error) error

func RunWithStorages(t *testing.T, fn func(*testing.T, storage.Reader, WithWriter)) {
	RunWithBadger(t, fn)
	RunWithPebble(t, fn)
}

func RunWithDB(t *testing.T, fn func(*testing.T, storage.DB)) {
	t.Run("BadgerStorage", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			fn(t, badgerimpl.ToDB(db))
		})
	})

	t.Run("PebbleStorage", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			fn(t, pebbleimpl.ToDB(db))
		})
	})
}

// RunFuncsWithNewPebbleDBHandle runs provided functions with
// new database handles of the same underlying database.
// Each provided function will receive a new (different) DB handle.
// This can be used to test database persistence.
func RunFuncsWithNewDBHandle(t *testing.T, fn ...func(*testing.T, storage.DB)) {
	t.Run("BadgerStorage", func(t *testing.T) {
		RunFuncsWithNewBadgerDBHandle(t, fn...)
	})

	t.Run("PebbleStorage", func(t *testing.T) {
		RunFuncsWithNewPebbleDBHandle(t, fn...)
	})
}

func RunWithBadger(t *testing.T, fn func(*testing.T, storage.Reader, WithWriter)) {
	t.Run("BadgerStorage", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, runWithBadger(func(r storage.Reader, wr WithWriter) {
			fn(t, r, wr)
		}))
	})
}

func RunWithPebble(t *testing.T, fn func(*testing.T, storage.Reader, WithWriter)) {
	t.Run("PebbleStorage", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, runWithPebble(func(r storage.Reader, wr WithWriter) {
			fn(t, r, wr)
		}))
	})
}

func RunWithPebbleDB(t *testing.T, opts *pebble.Options, fn func(*testing.T, storage.Reader, WithWriter, string, *pebble.DB)) {
	t.Run("PebbleStorage", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			db, err := pebble.Open(dir, opts)
			require.NoError(t, err)
			defer func() {
				require.NoError(t, db.Close())
			}()

			runWithPebble(func(r storage.Reader, w WithWriter) {
				fn(t, r, w, dir, db)
			})(db)
		})
	})
}

func BenchWithStorages(t *testing.B, fn func(*testing.B, storage.Reader, WithWriter)) {
	t.Run("BadgerStorage", func(t *testing.B) {
		unittest.RunWithBadgerDB(t, runWithBadger(func(r storage.Reader, wr WithWriter) {
			fn(t, r, wr)
		}))
	})

	t.Run("PebbleStorage", func(t *testing.B) {
		unittest.RunWithPebbleDB(t, runWithPebble(func(r storage.Reader, wr WithWriter) {
			fn(t, r, wr)
		}))
	})
}

func runWithBadger(fn func(storage.Reader, WithWriter)) func(*badger.DB) {
	return func(db *badger.DB) {
		withWriter := func(writing func(storage.Writer) error) error {
			return badgerimpl.ToDB(db).WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return writing(rw.Writer())
			})
		}

		reader := badgerimpl.ToReader(db)
		fn(reader, withWriter)
	}
}

func runWithPebble(fn func(storage.Reader, WithWriter)) func(*pebble.DB) {
	return func(db *pebble.DB) {
		withWriter := func(writing func(storage.Writer) error) error {
			return pebbleimpl.ToDB(db).WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return writing(rw.Writer())
			})
		}

		reader := pebbleimpl.ToReader(db)
		fn(reader, withWriter)
	}
}

// RunFuncsWithNewBadgerDBHandle runs provided functions with
// new BadgerDB handles of the same underlying database.
// Each provided function will receive a new (different) DB handle.
// This can be used to test database persistence.
func RunFuncsWithNewBadgerDBHandle(t *testing.T, fs ...func(*testing.T, storage.DB)) {
	unittest.RunWithTempDir(t, func(dir string) {
		// Run provided functions with new DB handles of the same underlying database.
		for _, f := range fs {
			// Open BadgerDB
			db := unittest.BadgerDB(t, dir)

			// Run provided function
			f(t, badgerimpl.ToDB(db))

			// Close BadgerDB
			assert.NoError(t, db.Close())
		}
	})
}

// RunFuncsWithNewPebbleDBHandle runs provided functions with
// new Pebble handles of the same underlying database.
// Each provided function will receive a new (different) DB handle.
// This can be used to test database persistence.
func RunFuncsWithNewPebbleDBHandle(t *testing.T, fs ...func(*testing.T, storage.DB)) {
	unittest.RunWithTempDir(t, func(dir string) {
		// Run provided f with new DB handle to test database persistence.
		for _, f := range fs {
			// Open Pebble
			db, err := pebble.Open(dir, &pebble.Options{})
			require.NoError(t, err)

			// Call provided function
			f(t, pebbleimpl.ToDB(db))

			// Close Pebble
			assert.NoError(t, db.Close())
		}
	})
}
