package backend_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	streammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
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
	timeout := state_stream.DefaultSendTimeout

	sub := streammock.NewStreamable(t)
	sub.On("ID").Return(uuid.NewString())

	tests := []testData{}
	for i := 0; i < 4; i++ {
		tests = append(tests, testData{fmt.Sprintf("test%d", i), nil})
	}
	tests = append(tests, testData{"", testErr})

	broadcaster := engine.NewBroadcaster()
	streamer := backend.NewStreamer(unittest.Logger(), broadcaster, timeout, state_stream.DefaultResponseLimit, sub)

	for _, d := range tests {
		sub.On("Next", mock.Anything).Return(d.data, d.err).Once()
		if d.err == nil {
			sub.On("Send", mock.Anything, d.data, timeout).Return(nil).Once()
		} else {
			mocked := sub.On("Fail", mock.Anything).Return().Once()
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
	timeout := state_stream.DefaultSendTimeout
	duration := 100 * time.Millisecond

	for _, limit := range []float64{0.2, 3, 20, 500} {
		t.Run(fmt.Sprintf("responses are limited - %.1f rps", limit), func(t *testing.T) {
			sub := streammock.NewStreamable(t)
			sub.On("ID").Return(uuid.NewString())

			broadcaster := engine.NewBroadcaster()
			streamer := backend.NewStreamer(unittest.Logger(), broadcaster, timeout, limit, sub)

			var nextCalls, sendCalls int
			sub.On("Next", mock.Anything).Return("data", nil).Run(func(args mock.Arguments) {
				nextCalls++
			})
			sub.On("Send", mock.Anything, "data", timeout).Return(nil).Run(func(args mock.Arguments) {
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
	timeout := state_stream.DefaultSendTimeout

	limit := 5.0
	duration := 30 * time.Second

	sub := streammock.NewStreamable(t)
	sub.On("ID").Return(uuid.NewString())

	broadcaster := engine.NewBroadcaster()
	streamer := backend.NewStreamer(unittest.Logger(), broadcaster, timeout, limit, sub)

	var nextCalls, sendCalls int
	sub.On("Next", mock.Anything).Return("data", nil).Run(func(args mock.Arguments) {
		nextCalls++
	})
	sub.On("Send", mock.Anything, "data", timeout).Return(nil).Run(func(args mock.Arguments) {
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
