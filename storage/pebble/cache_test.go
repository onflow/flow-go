package badger

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCache_Exists tests existence checking items in the cache.
func TestCache_Exists(t *testing.T) {
	cache := newCache[flow.Identifier, any](metrics.NewNoopCollector(), "test")

	t.Run("non-existent", func(t *testing.T) {
		key := unittest.IdentifierFixture()
		exists := cache.IsCached(key)
		assert.False(t, exists)
	})

	t.Run("existent", func(t *testing.T) {
		key := unittest.IdentifierFixture()
		cache.Insert(key, unittest.RandomBytes(128))

		exists := cache.IsCached(key)
		assert.True(t, exists)
	})

	t.Run("removed", func(t *testing.T) {
		key := unittest.IdentifierFixture()
		// insert, then remove the item
		cache.Insert(key, unittest.RandomBytes(128))
		cache.Remove(key)

		exists := cache.IsCached(key)
		assert.False(t, exists)
	})
}
