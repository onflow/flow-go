package state_stream_test

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
	streammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type testData struct {
	data string
	err  error
}

var testErr = fmt.Errorf("test error")

func TestStream(t *testing.T) {
	ctx := context.Background()
	timeout := state_stream.DefaultSendTimeout

	t.Run("happy path", func(t *testing.T) {
		sub := streammock.NewStreamable(t)
		sub.On("ID").Return(uuid.NewString())

		tests := []testData{}
		for i := 0; i < 4; i++ {
			tests = append(tests, testData{fmt.Sprintf("test%d", i), nil})
		}
		tests = append(tests, testData{"", testErr})

		broadcaster := engine.NewBroadcaster()
		streamer := state_stream.NewStreamer(unittest.Logger(), broadcaster, timeout, state_stream.DefaultThrottleDelay, sub)

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
		}, 10*time.Millisecond, "streamer.Stream() should return quickly")
	})

	t.Run("responses are throttled", func(t *testing.T) {
		sub := streammock.NewStreamable(t)
		sub.On("ID").Return(uuid.NewString())

		broadcaster := engine.NewBroadcaster()
		streamer := state_stream.NewStreamer(unittest.Logger(), broadcaster, timeout, 25*time.Millisecond, sub)

		sub.On("Next", mock.Anything).Return("data", nil).Times(2)
		sub.On("Send", mock.Anything, "data", timeout).Return(nil).Times(2)

		broadcaster.Publish()

		unittest.RequireNeverReturnBefore(t, func() {
			streamer.Stream(ctx)
		}, 40*time.Millisecond, "streamer.Stream() should take longer that 40ms")
	})
}
