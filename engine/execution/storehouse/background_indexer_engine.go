package storehouse

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/rs/zerolog"
)

type BackgroundIndexerEngine struct {
	component.Component
	log               zerolog.Logger
	newBlockExecuted  engine.Notifier
	backgroundIndexer *BackgroundIndexer
}

func NewBackgroundIndexerEngine(
	log zerolog.Logger,
	backgroundIndexer *BackgroundIndexer,
) *BackgroundIndexerEngine {

	b := &BackgroundIndexerEngine{
		log:               log,
		backgroundIndexer: backgroundIndexer,
		newBlockExecuted:  engine.NewNotifier(),
	}

	// Initialize the notifier so that even if no new data comes in,
	// the worker loop can still be triggered to process any existing data.
	b.newBlockExecuted.Notify()

	// Build component manager with worker loop
	cm := component.NewComponentManagerBuilder().
		AddWorker(b.workerLoop).
		Build()

	b.Component = cm
	return b
}

func (b *BackgroundIndexerEngine) OnBlockExecuted() {
	b.newBlockExecuted.Notify()
}

func (b *BackgroundIndexerEngine) workerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.newBlockExecuted.Channel():
			err := b.backgroundIndexer.IndexToLatest(ctx)
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
				ctx.Throw(fmt.Errorf("background indexer failed to index to latest: %w", err))
				return
			}
		}
	}
}
