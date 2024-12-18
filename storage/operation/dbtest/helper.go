package dbtest

import (
	"path"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// helper types and functions
type WithWriter func(func(storage.Writer) error) error

func RunWithStorages(t *testing.T, fn func(*testing.T, storage.Reader, WithWriter)) {
	unittest.RunWithTempDir(t, func(dir string) {
		RunWithStoragesAt(t, dir, fn)
	})
}

func RunWithStoragesAt(t *testing.T, dir string, fn func(*testing.T, storage.Reader, WithWriter)) {
	t.Run("BadgerStorage", func(t *testing.T) {
		unittest.RunWithBadgerDBAt(t, path.Join(dir, "badgertest"), func(db *badger.DB) {
			withWriter := func(writing func(storage.Writer) error) error {
				writer := badgerimpl.NewReaderBatchWriter(db)
				err := writing(writer)
				if err != nil {
					return err
				}

				// now := time.Now()
				// 30-40 microseconds
				err = writer.Commit()
				// fmt.Println("badger batch.Commit time:", time.Since(now).Microseconds())
				if err != nil {
					return err
				}
				return nil
			}

			reader := badgerimpl.ToReader(db)
			fn(t, reader, withWriter)
		})
	})

	t.Run("PebbleStorage", func(t *testing.T) {
		unittest.RunWithPebbleDBAt(t, path.Join(dir, "pebbletest"), func(db *pebble.DB) {
			withWriter := func(writing func(storage.Writer) error) error {
				writer := pebbleimpl.NewReaderBatchWriter(db)
				err := writing(writer)
				if err != nil {
					return err
				}

				// now := time.Now()
				// 3000-4000 microseconds with pebble.Batch updates
				err = writer.Commit()
				// fmt.Println("pebble batch.Commit time:", time.Since(now).Microseconds())
				if err != nil {
					return err
				}
				return nil
			}

			reader := pebbleimpl.ToReader(db)
			fn(t, reader, withWriter)
		})
	})
}

func BenchWithDB(b *testing.B, fn func(*testing.B, storage.DB)) {
	b.Run("BadgerStorage", func(b *testing.B) {
		unittest.RunWithBadgerDB(b, func(db *badger.DB) {
			fn(b, badgerimpl.ToDB(db))
		})
	})

	b.Run("PebbleStorage", func(b *testing.B) {
		unittest.RunWithPebbleDB(b, func(db *pebble.DB) {
			fn(b, pebbleimpl.ToDB(db))
		})
	})
}
