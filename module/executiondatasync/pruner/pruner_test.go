package pruner_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/executiondatasync/pruner"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
)

func TestBasicPrune(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(0), nil).Once()
	trackerStorage.On("GetPrunedHeight").Return(uint64(0), nil).Once()

	pruner, err := pruner.NewPruner(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		trackerStorage,
		pruner.WithHeightRangeTarget(10),
		pruner.WithThreshold(5),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	pruner.Start(signalerCtx)

	pruned := make(chan struct{})
	trackerStorage.On("Prune", uint64(6)).Return(func(height uint64) error {
		close(pruned)
		return nil
	}).Once()

	pruner.NotifyFulfilledHeight(16)
	<-pruned
	trackerStorage.AssertExpectations(t)

	cancel()
	<-pruner.Done()

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}

func TestInitialPrune(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(20), nil).Once()
	trackerStorage.On("GetPrunedHeight").Return(uint64(0), nil).Once()

	pruner, err := pruner.NewPruner(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		trackerStorage,
		pruner.WithHeightRangeTarget(10),
		pruner.WithThreshold(5),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	pruned := make(chan struct{})
	trackerStorage.On("Prune", uint64(10)).Return(func(height uint64) error {
		close(pruned)
		return nil
	}).Once()

	pruner.Start(signalerCtx)
	<-pruned
	trackerStorage.AssertExpectations(t)

	cancel()
	<-pruner.Done()

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}

func TestUpdateThreshold(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(15), nil).Once()
	trackerStorage.On("GetPrunedHeight").Return(uint64(0), nil).Once()

	pruner, err := pruner.NewPruner(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		trackerStorage,
		pruner.WithHeightRangeTarget(10),
		pruner.WithThreshold(10),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	pruner.Start(signalerCtx)

	pruned := make(chan struct{})
	trackerStorage.On("Prune", uint64(5)).Return(func(height uint64) error {
		close(pruned)
		return nil
	}).Once()

	pruner.SetThreshold(4)
	<-pruned
	trackerStorage.AssertExpectations(t)

	cancel()
	<-pruner.Done()

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}

func TestUpdateHeightRangeTarget(t *testing.T) {
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("GetFulfilledHeight").Return(uint64(10), nil).Once()
	trackerStorage.On("GetPrunedHeight").Return(uint64(0), nil).Once()

	pruner, err := pruner.NewPruner(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		trackerStorage,
		pruner.WithHeightRangeTarget(15),
		pruner.WithThreshold(0),
	)
	require.NoError(t, err)
	trackerStorage.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	pruner.Start(signalerCtx)

	pruned := make(chan struct{})
	trackerStorage.On("Prune", uint64(5)).Return(func(height uint64) error {
		close(pruned)
		return nil
	}).Once()

	pruner.SetHeightRangeTarget(5)
	<-pruned
	trackerStorage.AssertExpectations(t)

	cancel()
	<-pruner.Done()

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}
