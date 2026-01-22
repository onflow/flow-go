package storehouse

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// BackgroundIndexerEngine indexes register updates to storehouse for each executed and finalized blocks.
// "background" means that it runs in a separate worker loop and does not block the startup or the block
// execution.
type BackgroundIndexerEngine struct {
	component.Component
	log zerolog.Logger
	// since the indexer indexes for executed and finalized blocks,
	// we use a combined notifier to listen for the events and trigger the indexing.
	newBlockExecutedOrFinalized engine.Notifier
	// initializes the register store database by importing the root checkpoint,
	// which is then used to create the background indexer.
	bootstrapper func(ctx context.Context) (*BackgroundIndexer, io.Closer, error)
}

// newFinalizedAndExecutedNotifier creates a notifier that notifies when either a block is executed or finalized.
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

// NewBackgroundIndexerEngine creates a new BackgroundIndexerEngine.
func NewBackgroundIndexerEngine(
	log zerolog.Logger,
	bootstrapper func(ctx context.Context) (*BackgroundIndexer, io.Closer, error),
	blockExecutedNotifier BlockExecutedNotifier,
	followerDistributor *pubsub.FollowerDistributor,
) *BackgroundIndexerEngine {
	// blockExecutedNotifier notifies when a block is executed
	// followerDistributor notifies when a block is finalized
	// we combine both notifiers to trigger indexing on either event
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

// The background indexer engine runs  worker loop to kick off the bootstrapping process,
// then listens for new executed or finalized blocks to trigger indexing.
func (b *BackgroundIndexerEngine) workerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	b.log.Info().Msg("bootstrapping register store in background")

	backgroundIndexer, closer, err := b.bootstrapper(ctx)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to bootstrap background indexer: %w", err))
		return
	}
	// ensure the register store database is closed when the worker loop exits
	defer func() {
		if err := closer.Close(); err != nil {
			b.log.Error().Err(err).Msg("failed to close register store database")
		}
	}()

	b.log.Info().Msg("bootstrapping completed, starting background indexer worker loop")

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.newBlockExecutedOrFinalized.Channel():
			// the background indexer is
			err := backgroundIndexer.IndexUpToLatestFinalizedAndExecutedHeight(ctx)
			if err != nil {
				// If the error is context.Canceled and the parent context is also done,
				// it's likely due to termination/shutdown, so handle gracefully.
				// Otherwise, throw the error as it indicates a real problem.
				// TODO (leo): extract into a reusable function
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
