package cargo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/cargo"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFinalizedBlockTracker(t *testing.T) {

	t.Run("complience check", func(t *testing.T) {
		var err error
		capacity := 10
		headers := unittest.BlockHeaderFixtures(13)
		genesis, batch1, batch2, batch3 := headers[0], headers[1:5], headers[5:9], headers[9:]

		tracker := cargo.NewFinalizedBlockTracker(capacity, genesis)

		// add the first batch of headers
		for _, header := range batch1 {
			err = tracker.Enqueue(header)
			require.NoError(t, err)
		}

		// dequeue them all all
		for _, header := range batch1 {
			retID, retHeader := tracker.Peak()
			require.Equal(t, header.ID(), retID)
			require.Equal(t, header, retHeader)

			tracker.Dequeue()
		}

		// add the second batch of headers
		for _, header := range batch2 {
			err = tracker.Enqueue(header)
			require.NoError(t, err)
		}

		// pop one
		tracker.Dequeue()

		// trying adding the second batch again should fail
		for _, header := range batch2 {
			err = tracker.Enqueue(header)
			require.Error(t, err)
		}

		// add the third batch of headers
		for _, header := range batch3 {
			err = tracker.Enqueue(header)
			require.NoError(t, err)
		}
	})

	t.Run("capacity check", func(t *testing.T) {
		var err error
		capacity := 5
		headers := unittest.BlockHeaderFixtures(1 + capacity + 1)
		genesis, batch1, invalid := headers[0], headers[1:6], headers[6]

		tracker := cargo.NewFinalizedBlockTracker(capacity, genesis)
		// should be enough space for all of the
		for _, header := range batch1 {
			err = tracker.Enqueue(header)
			require.NoError(t, err)
		}

		err = tracker.Enqueue(invalid)
		require.Error(t, err)
	})
}
