package common

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
)

func InitStoragePebble(datadir string) (*pebble.DB, error) {
	return pebble.Open(datadir, nil)
}

func InitStoragesPebble(db *pebble.DB) *storage.All {
	metrics := &metrics.NoopCollector{}

	return storagepebble.InitAll(metrics, db)
}
