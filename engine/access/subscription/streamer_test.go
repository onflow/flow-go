package subscription_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	submock "github.com/onflow/flow-go/engine/access/subscription/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type testData struct {
	data string
	err  error
}

var testErr = fmt.Errorf("test error")

func TestStream(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	timeout := subscription.DefaultSendTimeout

	sub := submock.NewStreamable(t)
	sub.
		On("ID").
		Return(uuid.NewString())

	tests := []testData{}
	for i := 0; i < 4; i++ {
		tests = append(tests, testData{fmt.Sprintf("test%d", i), nil})
	}
	tests = append(tests, testData{"", testErr})

	broadcaster := engine.NewBroadcaster()

	// return test data sequentially by height. heights will start from 0 for this test
	dataByHeight := map[uint64]testData{}
	for i, d := range tests {
		dataByHeight[uint64(i)] = d
	}

	nextHeight := uint64(0)
	dataProvider := submock.NewDataProvider(t)
	dataProvider.
		On("NextData", mock.Anything).
		Return(func(ctx context.Context) (interface{}, error) {
			if td, ok := dataByHeight[nextHeight]; ok {
				nextHeight++
				if td.err != nil {
					return nil, td.err
				}

				return td.data, nil
			}

			// default to block not ready once we run out of prepared data
			return nil, subscription.ErrBlockNotReady
		})

	streamer := subscription.NewStreamer(
		unittest.Logger(),
		broadcaster,
		timeout,
		subscription.DefaultResponseLimit,
		sub,
		dataProvider,
	)

	for _, d := range tests {
		if d.err == nil {
			sub.
				On("Send", mock.Anything, d.data, timeout).
				Return(nil).
				Once()
		} else {
			mocked := sub.
				On("Fail", mock.Anything).
				Return().
				Once()

			mocked.RunFn = func(args mock.Arguments) {
				assert.ErrorIs(t, args.Get(0).(error), d.err)
			}
		}
	}

	broadcaster.Publish()

	unittest.RequireReturnsBefore(t, func() {
		streamer.Stream(ctx)
	}, 100*time.Millisecond, "streamer.Stream() should return quickly")
}

func TestStreamRatelimited(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	timeout := subscription.DefaultSendTimeout
	duration := 100 * time.Millisecond

	for _, limit := range []float64{0.2, 3, 20, 500} {
		t.Run(fmt.Sprintf("responses are limited - %.1f rps", limit), func(t *testing.T) {
			sub := submock.NewStreamable(t)
			sub.
				On("ID").
				Return(uuid.NewString())

			broadcaster := engine.NewBroadcaster()

			var nextCalls, sendCalls int

			dataProvider := submock.NewDataProvider(t)
			dataProvider.
				On("NextData", mock.Anything).
				Return(func(ctx context.Context) (interface{}, error) {
					nextCalls++
					return "data", nil
				})

			streamer := subscription.NewStreamer(
				unittest.Logger(),
				broadcaster,
				timeout,
				limit,
				sub,
				dataProvider,
			)

			sub.
				On("Send", mock.Anything, "data", timeout).
				Return(nil).
				Run(func(args mock.Arguments) {
					sendCalls++
				})

			broadcaster.Publish()

			unittest.RequireNeverReturnBefore(t, func() {
				streamer.Stream(ctx)
			}, duration, "streamer.Stream() should never stop")

			// check the number of calls and make sure they are sane.
			// ratelimit uses a token bucket algorithm which adds 1 token every 1/r seconds. This
			// comes to roughly 10% of r within 100ms.
			//
			// Add a large buffer since the algorithm only guarantees the rate over longer time
			// ranges. Since this test covers various orders of magnitude, we can still validate it
			// is working as expected.
			target := int(limit * float64(duration) / float64(time.Second))
			if target == 0 {
				target = 1
			}

			assert.LessOrEqual(t, nextCalls, target*3)
			assert.LessOrEqual(t, sendCalls, target*3)
		})
	}
}

// TestLongStreamRatelimited tests that the streamer is uses the correct rate limit over a longer
// period of time
func TestLongStreamRatelimited(t *testing.T) {
	t.Parallel()

	unittest.SkipUnless(t, unittest.TEST_LONG_RUNNING, "skipping long stream rate limit test")

	ctx := context.Background()
	timeout := subscription.DefaultSendTimeout

	limit := 5.0
	duration := 30 * time.Second

	sub := submock.NewStreamable(t)
	sub.
		On("ID").
		Return(uuid.NewString())

	broadcaster := engine.NewBroadcaster()

	var nextCalls, sendCalls int
	getData := func(ctx context.Context, height uint64) (interface{}, error) {
		nextCalls++
		return "data", nil
	}

	dataProvider := subscription.NewHeightByFuncProvider(0, getData)
	streamer := subscription.NewStreamer(unittest.Logger(), broadcaster, timeout, limit, sub, dataProvider)

	sub.
		On("Send", mock.Anything, "data", timeout).
		Return(nil).
		Run(func(args mock.Arguments) {
			sendCalls++
		})

	broadcaster.Publish()

	unittest.RequireNeverReturnBefore(t, func() {
		streamer.Stream(ctx)
	}, duration, "streamer.Stream() should never stop")

	// check the number of calls and make sure they are sane.
	// over a longer time, the rate limit should be more accurate
	target := int(limit) * int(duration/time.Second)
	diff := 5 // 5 ~= 3% of 150 expected

	assert.LessOrEqual(t, nextCalls, target+diff)
	assert.GreaterOrEqual(t, nextCalls, target-diff)

	assert.LessOrEqual(t, sendCalls, target+diff)
	assert.GreaterOrEqual(t, sendCalls, target-diff)
}
