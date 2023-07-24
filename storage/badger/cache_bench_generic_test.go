package badger

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func BenchmarkInterface(B *testing.B) {
	key := unittest.IdentifierFixture()
	cache := newCacheIFace(metrics.NewNoopCollector(), "test")

	for i := 0; i < B.N; i++ {
		// insert, then remove the item
		cache.Insert(key, unittest.RandomBytes(128))
		cache.Remove(key)

		_ = cache.IsCached(key)

	}
}

func BenchmarkGenerics(B *testing.B) {
	key := unittest.IdentifierFixture()
	cache := newCache[flow.Identifier, any](metrics.NewNoopCollector(), "test")

	for i := 0; i < B.N; i++ {
		// insert, then remove the item
		cache.Insert(key, unittest.RandomBytes(128))
		cache.Remove(key)

		_ = cache.IsCached(key)

	}
}
