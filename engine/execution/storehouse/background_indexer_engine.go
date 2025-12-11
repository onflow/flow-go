package storehouse

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/rs/zerolog"
)

type BackgroundIndexerEngine struct {
	component.Component
	log                         zerolog.Logger
	newBlockExecutedOrFinalized engine.Notifier
	bootstrapper                func(ctx context.Context) (*BackgroundIndexer, io.Closer, error)
	registerStoreCloser         io.Closer
}

func newFinalizedAndExecutedNotifier(
	blockExecutedNotifier BlockExecutedNotifier,
	followerDistributor *pubsub.FollowerDistributor,
) engine.Notifier {
	notifier := engine.NewNotifier()

	blockExecutedNotifier.AddConsumer(func() {
		notifier.Notify()
	})

	// Subscribe to block finalized events from the follower distributor
	followerDistributor.AddOnBlockFinalizedConsumer(func(_ *model.Block) {
		notifier.Notify()
	})

	return notifier
}

func NewBackgroundIndexerEngine(
	log zerolog.Logger,
	bootstrapper func(ctx context.Context) (*BackgroundIndexer, io.Closer, error),
	blockExecutedNotifier BlockExecutedNotifier,
	followerDistributor *pubsub.FollowerDistributor,
) *BackgroundIndexerEngine {
	finalizedOrExecutedNotifier := newFinalizedAndExecutedNotifier(blockExecutedNotifier, followerDistributor)

	b := &BackgroundIndexerEngine{
		log:                         log,
		bootstrapper:                bootstrapper,
		newBlockExecutedOrFinalized: finalizedOrExecutedNotifier,
	}

	// Initialize the notifier so that even if no new data comes in,
	// the worker loop can still be triggered to process any existing data.
	finalizedOrExecutedNotifier.Notify()

	// Build component manager with worker loop
	cm := component.NewComponentManagerBuilder().
		AddWorker(b.workerLoop).
		Build()

	b.Component = cm
	return b
}

func (b *BackgroundIndexerEngine) workerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	b.log.Info().Msg("bootstrapping register store in background")
	backgroundIndexer, closer, err := b.bootstrapper(ctx)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to bootstrap background indexer: %w", err))
		return
	}

	// Store the closer to close it during shutdown
	b.registerStoreCloser = closer

	b.log.Info().Msg("bootstrapping completed, starting background indexer worker loop")

	for {
		select {
		case <-ctx.Done():
			// Close the register store when shutting down
			if err := b.registerStoreCloser.Close(); err != nil {
				b.log.Error().Err(err).Msg("failed to close register store during shutdown")
			}
			return
		case <-b.newBlockExecutedOrFinalized.Channel():
			err := backgroundIndexer.IndexUpToLatestFinalizedAndExecutedHeight(ctx)
			if err != nil {
				// If the error is context.Canceled and the parent context is also done,
				// it's likely due to termination/shutdown, so handle gracefully.
				// Otherwise, throw the error as it indicates a real problem.
				if errors.Is(err, context.Canceled) && ctx.Err() != nil {
					// Cancellation due to termination - handle gracefully
					b.log.Warn().Msg("background indexer worker loop terminating due to context cancellation")
					return
				}
				// All other errors (including unexpected cancellations) should be thrown
				ctx.Throw(fmt.Errorf("background indexer failed to index up to latest finalized and executed height: %w", err))
				return
			}
		}
	}
}
