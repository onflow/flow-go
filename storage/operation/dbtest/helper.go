package dbtest

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
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
			writer := badgerimpl.NewReaderBatchWriter(db)
			err := writing(writer)
			if err != nil {
				return err
			}

			err = writer.Commit()
			if err != nil {
				return err
			}
			return nil
		}

		reader := badgerimpl.ToReader(db)
		fn(reader, withWriter)
	}
}

func runWithPebble(fn func(storage.Reader, WithWriter)) func(*pebble.DB) {
	return func(db *pebble.DB) {
		withWriter := func(writing func(storage.Writer) error) error {
			writer := pebbleimpl.NewReaderBatchWriter(db)
			err := writing(writer)
			if err != nil {
				return err
			}

			err = writer.Commit()
			if err != nil {
				return err
			}
			return nil
		}

		reader := pebbleimpl.ToReader(db)
		fn(reader, withWriter)
	}
}
