package internal_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewSubscriptionRecordCache tests that NewSubscriptionRecordCache returns a valid cache.
func TestNewSubscriptionRecordCache(t *testing.T) {
	sizeLimit := uint32(100)

	cache := internal.NewSubscriptionRecordCache(
		sizeLimit,
		unittest.Logger(),
		metrics.NewSubscriptionRecordCacheMetricsFactory(metrics.NewNoopHeroCacheMetricsFactory()))

	require.NotNil(t, cache, "cache should not be nil")
	require.IsType(t, &internal.SubscriptionRecordCache{}, cache, "cache should be of type *SubscriptionRecordCache")
}
