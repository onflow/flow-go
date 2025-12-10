package storehouse

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

type RegisterUpdatesProvider interface {
	LatestHeight() uint64
	RegisterUpdatesByHeight(height uint64) (flow.RegisterEntries, error)
}

type BackgroundIndexer struct {
	provider RegisterUpdatesProvider
	headers  storage.Headers

	registerStore execution.RegisterStore
}

func (b *BackgroundIndexer) IndexToLatest() error {
	startHeight := b.registerStore.LastFinalizedAndExecutedHeight()
	endHeight := b.provider.LatestHeight()

	// If startHeight is already at or beyond endHeight, nothing to do
	if startHeight >= endHeight {
		return nil
	}

	// Loop through each height from startHeight+1 to endHeight
	for h := startHeight + 1; h <= endHeight; h++ {
		// Get register entries for this height
		registerEntries, err := b.provider.RegisterUpdatesByHeight(h)
		if err != nil {
			return fmt.Errorf("failed to get register entries for height %d: %w", h, err)
		}

		header, err := b.headers.ByHeight(h)
		if err != nil {
			return fmt.Errorf("failed to get header for height %d: %w", h, err)
		}

		// Store registers directly to disk store for background indexing
		err = b.registerStore.SaveRegisters(header, registerEntries)
		if err != nil {
			return fmt.Errorf("failed to store registers for height %d: %w", h, err)
		}
	}

	return nil
}

type BackgroundIndexerEngine struct {
	component.Component
	backgroundIndexer *BackgroundIndexer
	newBlockExecuted  engine.Notifier
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
			b.backgroundIndexer.IndexToLatest()
		}
	}
}
