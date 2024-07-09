package unittest

import "github.com/cockroachdb/pebble"

func PebbleUpdate(db *pebble.DB, fn func(tx *pebble.Batch) error) error {
	batch := db.NewBatch()
	err := fn(batch)
	if err != nil {
		return err
	}
	return batch.Commit(nil)
}
