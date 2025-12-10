package storehouse

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/rs/zerolog"
)

type BackgroundIndexerEngine struct {
	component.Component
	newBlockExecuted  engine.Notifier
	backgroundIndexer *BackgroundIndexer
}

func NewBackgroundIndexerEngine(
	log zerolog.Logger,
	backgroundIndexer *BackgroundIndexer,
) *BackgroundIndexerEngine {

	b := &BackgroundIndexerEngine{
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
				// TODO: only throw if is cancellation due to termination
				ctx.Throw(fmt.Errorf("background indexer failed to index to latest: %w", err))
				return
			}
		}
	}
}
