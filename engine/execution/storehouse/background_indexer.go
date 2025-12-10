package storehouse

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type RegisterUpdatesProvider interface {
	RegisterUpdatesByHeight(ctx context.Context, blockID flow.Identifier) (flow.RegisterEntries, bool, error)
}

type BackgroundIndexer struct {
	provider      RegisterUpdatesProvider
	registerStore execution.RegisterStore
	state         protocol.State
	headers       storage.Headers
}

func NewBackgroundIndexer(
	provider RegisterUpdatesProvider,
	registerStore execution.RegisterStore,
	state protocol.State,
	headers storage.Headers,
) *BackgroundIndexer {
	return &BackgroundIndexer{
		provider:      provider,
		registerStore: registerStore,
		state:         state,
		headers:       headers,
	}
}

func (b *BackgroundIndexer) IndexToLatest(ctx context.Context) error {
	startHeight := b.registerStore.LastFinalizedAndExecutedHeight()
	latestFinalized, err := b.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get latest finalized height: %w", err)
	}

	// Loop through each unindexed finalized height, fetch register updates and store them
	for h := startHeight + 1; h <= latestFinalized.Height; h++ {
		header, err := b.headers.ByHeight(h)
		if err != nil {
			return fmt.Errorf("failed to get header for height %d: %w", h, err)
		}

		// Get register entries for this height
		registerEntries, executed, err := b.provider.RegisterUpdatesByHeight(ctx, header.ID())
		if err != nil {
			return fmt.Errorf("failed to get register entries for height %d: %w", h, err)
		}

		if !executed {
			return nil
		}

		// Store registers directly to disk store for background indexing
		err = b.registerStore.SaveRegisters(header, registerEntries)
		if err != nil {
			return fmt.Errorf("failed to store registers for height %d: %w", h, err)
		}
	}

	return nil
}
