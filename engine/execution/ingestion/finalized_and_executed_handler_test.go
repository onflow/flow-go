package ingestion

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func Test_finalizedAndExecutedDispatcher_callsForwardedCorrectly(t *testing.T) {
	ctx := context.Background()

	latestFinalizedAndExecutedHeight := uint64(100)
	isBlockFinalizedCalled := 0
	isBlockFinalized := func(context.Context, *flow.Header) (bool, error) {
		isBlockFinalizedCalled++
		return true, nil
	}
	isBlockExecutedCalled := 0
	isBlockExecuted := func(context.Context, *flow.Header) (bool, error) {
		isBlockExecutedCalled++
		return true, nil
	}

	dispatcher := NewFinalizedAndExecutedDispatcher(
		latestFinalizedAndExecutedHeight,
		isBlockFinalized,
		isBlockExecuted,
	)

	consumerCalled := 0
	consumer := func(h *flow.Header) {
		consumerCalled++
	}
	dispatcher.AddOnFinalizedAndExecutedConsumer(consumer)

	err := dispatcher.OnBlockExecuted(ctx, header(102))
	require.NoError(t, err)
	// non sequentially executed block
	require.Equal(t, 0, isBlockFinalizedCalled)
	err = dispatcher.OnBlockExecuted(ctx, header(99))
	require.NoError(t, err)
	require.Equal(t, 0, isBlockFinalizedCalled)
	err = dispatcher.OnBlockExecuted(ctx, header(100))
	require.NoError(t, err)
	require.Equal(t, 0, isBlockFinalizedCalled)
	err = dispatcher.OnBlockExecuted(ctx, header(101))
	require.NoError(t, err)
	// this is now the new finalized and executed height
	require.Equal(t, 1, isBlockFinalizedCalled)
	require.Equal(t, 1, consumerCalled)

	err = dispatcher.OnBlockFinalized(ctx, header(103))
	require.NoError(t, err)
	// non sequentially finalized block
	err = dispatcher.OnBlockFinalized(ctx, header(100))
	require.NoError(t, err)
	require.Equal(t, 0, isBlockExecutedCalled)
	err = dispatcher.OnBlockFinalized(ctx, header(101))
	require.NoError(t, err)
	require.Equal(t, 0, isBlockExecutedCalled)
	err = dispatcher.OnBlockFinalized(ctx, header(102))
	require.NoError(t, err)
	// this is now the new finalized and executed height
	require.Equal(t, 1, isBlockExecutedCalled)
	require.Equal(t, 2, consumerCalled)
}

func Test_finalizedAndExecutedDispatcher_ErrorPropagation(t *testing.T) {
	ctx := context.Background()

	someError := errors.Errorf("some error")
	someError2 := errors.Errorf("some error 2")

	latestFinalizedAndExecutedHeight := uint64(100)
	isBlockFinalizedCalled := 0
	isBlockFinalized := func(context.Context, *flow.Header) (bool, error) {
		isBlockFinalizedCalled++
		return true, someError
	}
	isBlockExecutedCalled := 0
	isBlockExecuted := func(context.Context, *flow.Header) (bool, error) {
		isBlockExecutedCalled++
		return false, someError2
	}

	dispatcher := NewFinalizedAndExecutedDispatcher(
		latestFinalizedAndExecutedHeight,
		isBlockFinalized,
		isBlockExecuted,
	)

	err := dispatcher.OnBlockExecuted(ctx, header(101))
	require.ErrorIs(t, err, someError)

	err = dispatcher.OnBlockFinalized(ctx, header(101))
	require.ErrorIs(t, err, someError2)
}

func header(h uint64) *flow.Header {
	return &flow.Header{
		Height: h,
	}
}
