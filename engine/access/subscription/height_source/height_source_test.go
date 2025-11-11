package height_source

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	subpkg "github.com/onflow/flow-go/engine/access/subscription"
)

func TestHeightSource_ReadyAndGet(t *testing.T) {
	t.Parallel()

	readyCalled := 0
	var readyUpTo uint64 = 10
	hs := NewHeightSource[int](
		1,
		5,
		func() (uint64, error) {
			readyCalled++
			return readyUpTo, nil
		},
		func(ctx context.Context, h uint64) (int, error) { return int(h), nil },
	)

	assert.Equal(t, uint64(1), hs.StartHeight())
	assert.Equal(t, uint64(5), hs.EndHeight())

	r, err := hs.ReadyUpToHeight()
	require.NoError(t, err)
	assert.Equal(t, readyUpTo, r)
	assert.Equal(t, 1, readyCalled)

	ctx := context.Background()
	val, err := hs.GetItemAtHeight(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, 3, val)
}

func TestHeightSource_GetItemAtHeight_ContextCancel(t *testing.T) {
	t.Parallel()

	hs := NewHeightSource[int](
		0,
		0,
		func() (uint64, error) { return 0, nil },
		func(ctx context.Context, h uint64) (int, error) {
			// wait so we can cancel
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return int(h), nil
			}
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := hs.GetItemAtHeight(ctx, 1)
	require.Error(t, err)
}

func TestHeightSource_GetItemAtHeight_ErrorPropagation(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")

	hs := NewHeightSource[int](
		0,
		0,
		func() (uint64, error) { return 0, nil },
		func(ctx context.Context, h uint64) (int, error) { return 0, sentinel },
	)

	_, err := hs.GetItemAtHeight(context.Background(), 5)
	require.Error(t, err)
	// current implementation may wrap with ErrBlockNotReady; ensure we can still detect the sentinel
	assert.ErrorIs(t, err, sentinel)
	// and often also tag as not ingested; allow either behavior here
	_ = subpkg.ErrBlockNotReady
}
