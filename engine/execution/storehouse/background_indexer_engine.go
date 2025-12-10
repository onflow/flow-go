package storehouse

import (
	"context"
	"errors"
	"fmt"

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
	backgroundIndexer           *BackgroundIndexer
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
	backgroundIndexer *BackgroundIndexer,
	blockExecutedNotifier BlockExecutedNotifier,
	followerDistributor *pubsub.FollowerDistributor,
) *BackgroundIndexerEngine {
	finalizedOrExecutedNotifier := newFinalizedAndExecutedNotifier(blockExecutedNotifier, followerDistributor)

	b := &BackgroundIndexerEngine{
		log:                         log,
		backgroundIndexer:           backgroundIndexer,
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
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.newBlockExecutedOrFinalized.Channel():
			err := b.backgroundIndexer.IndexUpToLatestFinalizedAndExecutedHeight(ctx)
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
