package queue_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/cargo/queue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFinalizedBlockQueue(t *testing.T) {

	t.Run("complience check", func(t *testing.T) {
		var err error
		capacity := 10
		headers := unittest.BlockHeaderFixtures(13)
		genesis, batch1, batch2, batch3 := headers[0], headers[1:5], headers[5:9], headers[9:]

		bq := queue.NewFinalizedBlockQueue(capacity, genesis)

		// add the first batch of headers
		for _, header := range batch1 {
			err = bq.Enqueue(header)
			require.NoError(t, err)
		}

		// dequeue them all all
		for _, header := range batch1 {
			retID, retHeader := bq.Peak()
			require.Equal(t, header.ID(), retID)
			require.Equal(t, header, retHeader)

			bq.Dequeue()
		}

		// trying the third batch of headers should fail (in the future)
		for _, header := range batch3 {
			err = bq.Enqueue(header)
			require.Error(t, err)
			expectedError := &queue.NonCompliantHeaderHeightError{}
			require.True(t, errors.As(err, &expectedError))
		}

		// add the second batch of headers
		for _, header := range batch2 {
			err = bq.Enqueue(header)
			require.NoError(t, err)
		}

		// pop one
		bq.Dequeue()

		// trying adding the second batch again should fail (in the past)
		for _, header := range batch2 {
			err = bq.Enqueue(header)
			require.Error(t, err)
			expectedError := &queue.NonCompliantHeaderAlreadyProcessedError{}
			require.True(t, errors.As(err, &expectedError))
		}

		// add the third batch of headers
		for _, header := range batch3 {
			err = bq.Enqueue(header)
			require.NoError(t, err)
		}
	})

	t.Run("capacity check", func(t *testing.T) {
		var err error
		capacity := 4
		headers := unittest.BlockHeaderFixtures(1 + capacity + 1)
		genesis, batch1, invalid := headers[0], headers[1:5], headers[5]

		bq := queue.NewFinalizedBlockQueue(capacity, genesis)
		// should be enough space for all of the 4 headers
		for _, header := range batch1 {
			err = bq.Enqueue(header)
			require.NoError(t, err)
		}

		err = bq.Enqueue(invalid)
		require.Error(t, err)
		expectedError := &queue.QueueCapacityReachedError{}
		require.True(t, errors.As(err, &expectedError))
	})

	t.Run("test peak functionality", func(t *testing.T) {
		var err error
		capacity := 2

		headers := unittest.BlockHeaderFixtures(3)
		genesis, headers := headers[0], headers[1:]

		bq := queue.NewFinalizedBlockQueue(capacity, genesis)

		id, header := bq.Peak()
		require.Nil(t, header)
		require.Equal(t, flow.ZeroID, id)

		// insert the first one
		err = bq.Enqueue(headers[0])
		require.NoError(t, err)

		// now it should return the first header
		id, header = bq.Peak()
		require.Equal(t, headers[0], header)
		require.Equal(t, headers[0].ID(), id)

		// insert the second one
		err = bq.Enqueue(headers[1])
		require.NoError(t, err)

		// it should still return the first header
		id, header = bq.Peak()
		require.Equal(t, headers[0], header)
		require.Equal(t, headers[0].ID(), id)

		// dequeue
		bq.Dequeue()

		// now it should return the second header
		id, header = bq.Peak()
		require.Equal(t, headers[1], header)
		require.Equal(t, headers[1].ID(), id)

		// dequeue
		bq.Dequeue()

		// now should return empty
		id, header = bq.Peak()
		require.Nil(t, header)
		require.Equal(t, flow.ZeroID, id)
	})
}
