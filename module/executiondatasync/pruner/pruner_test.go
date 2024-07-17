package pruner_test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	exedatamock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/executiondatasync/pruner"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
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

	downloader := new(exedatamock.Downloader)

	var notifier *engine.Notifier
	downloader.On("Register", mock.Anything).Return().Once().Run(func(args mock.Arguments) {
		notifier = args.Get(0).(*engine.Notifier)
	})
	downloader.On("LastProcessedHeight").
		Return(uint64(16)).
		Once()

	pruner.RegisterProducer(downloader)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

	pruner.Start(signalerCtx)
	notifier.Notify()

	pruned := make(chan struct{})
	trackerStorage.On("PruneUpToHeight", uint64(6)).Return(func(height uint64) error {
		close(pruned)
		return nil
	}).Once()
	trackerStorage.On("SetFulfilledHeight", uint64(16)).Return(nil).Maybe()

	unittest.AssertClosesBefore(t, pruned, time.Second)
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
	trackerStorage.On("PruneUpToHeight", uint64(10)).Return(func(height uint64) error {
		close(pruned)
		return nil
	}).Once()

	pruner.Start(signalerCtx)
	unittest.AssertClosesBefore(t, pruned, time.Second)
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
	trackerStorage.On("PruneUpToHeight", uint64(5)).Return(func(height uint64) error {
		close(pruned)
		return nil
	}).Once()

	require.NoError(t, pruner.SetThreshold(4))
	unittest.AssertClosesBefore(t, pruned, time.Second)
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
	trackerStorage.On("PruneUpToHeight", uint64(5)).Return(func(height uint64) error {
		close(pruned)
		return nil
	}).Once()

	require.NoError(t, pruner.SetHeightRangeTarget(5))
	unittest.AssertClosesBefore(t, pruned, time.Second)
	trackerStorage.AssertExpectations(t)

	cancel()
	<-pruner.Done()

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}
