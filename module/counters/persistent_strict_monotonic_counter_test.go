package counters_test

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMonotonicConsumer(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		var height1 = uint64(1234)
		persistentStrictMonotonicCounter, err := counters.NewPersistentStrictMonotonicCounter(
			bstorage.NewConsumerProgress(db, module.ConsumeProgressLastFullBlockHeight),
			height1,
		)
		require.NoError(t, err)

		// check value can be retrieved
		actual := persistentStrictMonotonicCounter.Value()
		require.Equal(t, height1, actual)

		// try to update value with less than current
		var lessHeight = uint64(1233)
		err = persistentStrictMonotonicCounter.Set(lessHeight)
		require.Error(t, err)
		require.Equal(t, err, fmt.Errorf("could not update consumer progress to height that is lower than the current height"))

		// update the value with bigger height
		var height2 = uint64(1235)
		err = persistentStrictMonotonicCounter.Set(height2)
		require.NoError(t, err)

		// check that the new value can be retrieved
		actual = persistentStrictMonotonicCounter.Value()
		require.Equal(t, height2, actual)

		// check that new persistent strict monotonic counter has the same value
		persistentStrictMonotonicCounter2, err := counters.NewPersistentStrictMonotonicCounter(
			bstorage.NewConsumerProgress(db, module.ConsumeProgressLastFullBlockHeight),
			height1,
		)
		require.NoError(t, err)

		// check that the value still the same
		actual = persistentStrictMonotonicCounter2.Value()
		require.Equal(t, height2, actual)
	})
}
