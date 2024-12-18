package dbtest

import (
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
	t.Run("BadgerStorage", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
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
			fn(t, reader, withWriter)
		})
	})

	t.Run("PebbleStorage", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
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
			fn(t, reader, withWriter)
		})
	})
}
