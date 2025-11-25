package counters_test

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMonotonicConsumer(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		var height1 = uint64(1234)
		progress, err := store.NewConsumerProgress(pebbleimpl.ToDB(pdb), module.ConsumeProgressLastFullBlockHeight).Initialize(height1)
		require.NoError(t, err)
		persistentStrictMonotonicCounter, err := counters.NewPersistentStrictMonotonicCounter(progress)
		require.NoError(t, err)

		// check value can be retrieved
		actual := persistentStrictMonotonicCounter.Value()
		require.Equal(t, height1, actual)

		// try to update value with less than current
		var lessHeight = uint64(1233)
		err = persistentStrictMonotonicCounter.Set(lessHeight)
		require.Error(t, err)
		require.ErrorIs(t, err, counters.ErrIncorrectValue)

		// update the value with bigger height
		var height2 = uint64(1235)
		err = persistentStrictMonotonicCounter.Set(height2)
		require.NoError(t, err)

		// check that the new value can be retrieved
		actual = persistentStrictMonotonicCounter.Value()
		require.Equal(t, height2, actual)

		progress2, err := store.NewConsumerProgress(pebbleimpl.ToDB(pdb), module.ConsumeProgressLastFullBlockHeight).Initialize(height1)
		require.NoError(t, err)
		// check that new persistent strict monotonic counter has the same value
		persistentStrictMonotonicCounter2, err := counters.NewPersistentStrictMonotonicCounter(progress2)
		require.NoError(t, err)

		// check that the value still the same
		actual = persistentStrictMonotonicCounter2.Value()
		require.Equal(t, height2, actual)
	})
}
