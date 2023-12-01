package backend_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/engine/access/state_stream/backend"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestSubscription tests that the subscription forwards the data correctly and in order
func TestSubscription_SendReceive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	sub := backend.NewSubscription(1)

	assert.NotEmpty(t, sub.ID())

	messageCount := 20
	messages := []string{}
	for i := 0; i < messageCount; i++ {
		messages = append(messages, fmt.Sprintf("test messages %d", i))
	}
	receivedCount := 0

	wg := sync.WaitGroup{}
	wg.Add(1)

	// receive each message and validate it has the expected value
	go func() {
		defer wg.Done()

		for v := range sub.Channel() {
			assert.Equal(t, messages[receivedCount], v)
			receivedCount++
		}
	}()

	// send all messages in order
	for _, d := range messages {
		err := sub.Send(ctx, d, 10*time.Millisecond)
		require.NoError(t, err)
	}
	sub.Close()

	assert.NoError(t, sub.Err())

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "received never finished")

	assert.Equal(t, messageCount, receivedCount)
}

// TestSubscription_Failures tests closing and failing subscriptions behaves as expected
func TestSubscription_Failures(t *testing.T) {
	t.Parallel()

	testErr := fmt.Errorf("test error")

	// make sure closing a subscription twice does not cause a panic
	t.Run("close only called once", func(t *testing.T) {
		sub := backend.NewSubscription(1)
		sub.Close()
		sub.Close()

		assert.NoError(t, sub.Err())
	})

	// make sure failing and closing the same subscription does not cause a panic
	t.Run("close only called once with fail", func(t *testing.T) {
		sub := backend.NewSubscription(1)
		sub.Fail(testErr)
		sub.Close()

		assert.ErrorIs(t, sub.Err(), testErr)
	})

	// make sure an error is returned when sending on a closed subscription
	t.Run("send after closed returns an error", func(t *testing.T) {
		sub := backend.NewSubscription(1)
		sub.Fail(testErr)

		err := sub.Send(context.Background(), "test", 10*time.Millisecond)
		assert.Error(t, err, "expected subscription closed error")

		assert.ErrorIs(t, sub.Err(), testErr)
	})
}

// TestHeightBasedSubscription tests that the height based subscription tracks heights correctly
// and forwards the error correctly
func TestHeightBasedSubscription(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	start := uint64(3)
	last := uint64(10)

	errNoData := fmt.Errorf("no more data")

	next := start
	getData := func(_ context.Context, height uint64) (interface{}, error) {
		require.Equal(t, next, height)
		if height >= last {
			return nil, errNoData
		}
		next++
		return height, nil
	}

	// search from [start, last], checking the correct data is returned
	sub := backend.NewHeightBasedSubscription(1, start, getData)
	for i := start; i <= last; i++ {
		data, err := sub.Next(ctx)
		if err != nil {
			// after the last element is returned, next == last
			assert.Equal(t, last, next, "next should be equal to last")
			assert.ErrorIs(t, err, errNoData)
			break
		}

		require.Equal(t, i, data)
	}
}
