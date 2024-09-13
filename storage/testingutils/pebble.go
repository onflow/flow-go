package testingutils

import (
	"testing"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	pstorage "github.com/onflow/flow-go/storage/pebble"
)

func PebbleStorageLayer(_ testing.TB, db *pebble.DB) *storage.All {
	metrics := metrics.NewNoopCollector()
	all := pstorage.InitAll(metrics, db)
	return all
}
