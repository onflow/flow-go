package streamer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/height_source"
	subimpl "github.com/onflow/flow-go/engine/access/subscription/subscription"
	"github.com/onflow/flow-go/utils/unittest"
)

func collectUntilClosed[T any](ch <-chan T, timeout time.Duration) ([]T, error) {
	var out []T
	t := time.NewTimer(timeout)
	defer t.Stop()
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return out, nil
			}
			out = append(out, v)
		case <-t.C:
			return out, fmt.Errorf("timeout waiting for channel close")
		}
	}
}

func TestHeightBasedStreamer_BoundedHappyPath(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	start := uint64(3)
	end := uint64(10)

	// ready up to a large value
	ready := func() (uint64, error) { return 100, nil }
	get := func(ctx context.Context, h uint64) (int, error) { return int(h), nil }

	hs := height_source.NewHeightSource[int](start, end, ready, get)
	sub := subimpl.NewSubscription[int](64)

	bc := engine.NewBroadcaster()
	opts := NewDefaultStreamOptions()
	opts.Heartbeat = 50 * time.Millisecond
	opts.ResponseLimit = 0

	s := NewHeightBasedStreamer[int](unittest.Logger(), bc, sub, hs, opts)
	go s.Stream(ctx)

	// kick once
	bc.Publish()

	items, err := collectUntilClosed(sub.Channel(), 2*time.Second)
	require.NoError(t, err)

	// Expect [start..end]
	expected := make([]int, 0, end-start+1)
	for i := start; i <= end; i++ {
		expected = append(expected, int(i))
	}
	assert.Equal(t, expected, items)
	assert.NoError(t, sub.Err())
}

func TestHeightBasedStreamer_ErrorsHandling(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	start := uint64(1)
	end := uint64(5)

	sentinel := errors.New("boom")
	ready := func() (uint64, error) { return 100, nil }
	get := func(ctx context.Context, h uint64) (int, error) {
		if h == 3 {
			return 0, sentinel
		}
		return int(h), nil
	}

	hs := height_source.NewHeightSource[int](start, end, ready, get)
	sub := subimpl.NewSubscription[int](10)
	bc := engine.NewBroadcaster()
	opts := NewDefaultStreamOptions()
	opts.Heartbeat = 20 * time.Millisecond
	opts.ResponseLimit = 0

	s := NewHeightBasedStreamer[int](unittest.Logger(), bc, sub, hs, opts)
	go s.Stream(ctx)

	bc.Publish()

	// Read until channel closes
	_, err := collectUntilClosed(sub.Channel(), 2*time.Second)
	require.NoError(t, err)
	assert.ErrorIs(t, sub.Err(), sentinel)
}

func TestHeightBasedStreamer_NotIngestedThenAvailable(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	start := uint64(1)
	end := uint64(0) // unbounded

	var readyTo uint64 = 1 // only first item initially
	ready := func() (uint64, error) { return readyTo, nil }

	var nextErrHeight uint64 = 2
	get := func(ctx context.Context, h uint64) (int, error) {
		if h == nextErrHeight {
			return 0, subscription.ErrItemNotIngested
		}
		return int(h), nil
	}

	hs := height_source.NewHeightSource[int](start, end, ready, get)
	sub := subimpl.NewSubscription[int](10)
	bc := engine.NewBroadcaster()
	opts := NewDefaultStreamOptions()
	opts.Heartbeat = 50 * time.Millisecond
	opts.ResponseLimit = 0

	s := NewHeightBasedStreamer[int](unittest.Logger(), bc, sub, hs, opts)
	go s.Stream(ctx)

	bc.Publish()

	// Expect to receive h=1 then block on h=2 not ingested. After we advance readiness and clear error, more data should flow.
	select {
	case v := <-sub.Channel():
		assert.Equal(t, 1, v)
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("did not receive initial item")
	}

	// Now make height 2 available
	nextErrHeight = 0 // no longer error
	readyTo = 3       // allow up to 3
	bc.Publish()

	// Receive 2 and 3
	got2 := false
	got3 := false
	for !(got2 && got3) {
		select {
		case v := <-sub.Channel():
			if v == 2 {
				got2 = true
			}
			if v == 3 {
				got3 = true
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("did not receive follow-up items in time")
		}
	}

	// cancel the stream to stop
	cancel()
	// streamer should close with error set
	time.Sleep(50 * time.Millisecond)
	assert.Error(t, sub.Err())
}
