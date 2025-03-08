package chained

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExecutionResultsOnlyFirstHave(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		bresults := store.NewExecutionResults(metrics.NewNoopCollector(), badgerimpl.ToDB(bdb))
		presults := store.NewExecutionResults(metrics.NewNoopCollector(), pebbleimpl.ToDB(pdb))

		result := unittest.ExecutionResultFixture()

		chained := NewExecutionResults(presults, bresults)

		// not found
		_, err := chained.ByID(result.ID())
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// only stored in first
		require.NoError(t, presults.Store(result))
		actual, err := chained.ByID(result.ID())
		require.NoError(t, err)

		require.Equal(t, result, actual)
	})
}

func TestExecutionResultsOnlySecondHave(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		bresults := store.NewExecutionResults(metrics.NewNoopCollector(), badgerimpl.ToDB(bdb))
		presults := store.NewExecutionResults(metrics.NewNoopCollector(), pebbleimpl.ToDB(pdb))

		result := unittest.ExecutionResultFixture()

		chained := NewExecutionResults(presults, bresults)
		// only stored in second
		require.NoError(t, bresults.Store(result))
		actual, err := chained.ByID(result.ID())
		require.NoError(t, err)

		require.Equal(t, result, actual)
	})
}
