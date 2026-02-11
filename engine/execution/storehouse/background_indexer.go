package storehouse

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// RegisterUpdatesProvider defines an interface to fetch register updates for a given block ID.
type RegisterUpdatesProvider interface {
	RegisterUpdatesByBlockID(ctx context.Context, blockID flow.Identifier) (flow.RegisterEntries, bool, error)
}

const DefaultHeightsPerSecond = 0     // 0 means no rate limiting by default
const MaxHeightsPerSecond = 1_000_000 // hard cap to prevent ineffective rate limiting

// BackgroundIndexer indexes register updates for finalized and executed blocks.
// It is passive and runs only when triggered by the BackgroundIndexerEngine.
type BackgroundIndexer struct {
	log              zerolog.Logger
	registerStore    execution.RegisterStore // write register updates to database
	provider         RegisterUpdatesProvider // read register updates for each block
	state            protocol.State          // read last finalized height for iteration
	headers          storage.Headers         // read block headers by height, header is needed to store registers
	heightsPerSecond uint64                  // rate limit for indexing heights per second
}

func NewBackgroundIndexer(
	log zerolog.Logger,
	provider RegisterUpdatesProvider,
	registerStore execution.RegisterStore,
	state protocol.State,
	headers storage.Headers,
	heightsPerSecond uint64,
) *BackgroundIndexer {
	// Cap heightsPerSecond to prevent ineffective rate limiting
	if heightsPerSecond > MaxHeightsPerSecond {
		heightsPerSecond = MaxHeightsPerSecond
	}

	return &BackgroundIndexer{
		log:              log.With().Str("component", "background_indexer").Logger(),
		provider:         provider,
		registerStore:    registerStore,
		state:            state,
		headers:          headers,
		heightsPerSecond: heightsPerSecond,
	}
}

// IndexUpToLatestFinalizedAndExecutedHeight indexes register updates for each finalized
// and executed block, starting from the last indexed height up to the latest finalized and
// executed height.
func (b *BackgroundIndexer) IndexUpToLatestFinalizedAndExecutedHeight(ctx context.Context) error {
	lastIndexedHeight := b.registerStore.LastFinalizedAndExecutedHeight()
	latestFinalized, err := b.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get latest finalized height: %w", err)
	}

	b.log.Debug().
		Uint64("last_indexed_height", lastIndexedHeight).
		Uint64("latest_finalized_height", latestFinalized.Height).
		Uint64("heights_per_second", b.heightsPerSecond).
		Msg("indexing registers up to latest finalized and executed height")

	// Calculate sleep duration per height if rate limiting is enabled
	var sleepDuration time.Duration
	if b.heightsPerSecond > 0 {
		sleepDuration = time.Second / time.Duration(b.heightsPerSecond)
	}

	// Loop through each unindexed finalized height, fetch register updates and store them
	for h := lastIndexedHeight + 1; h <= latestFinalized.Height; h++ {
		// Check context cancellation before processing each height
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := b.headers.ByHeight(h)
		if err != nil {
			return fmt.Errorf("failed to get header for height %d: %w", h, err)
		}

		// Get register entries for this height
		registerEntries, executed, err := b.provider.RegisterUpdatesByBlockID(ctx, header.ID())
		if err != nil {
			return fmt.Errorf("failed to get register entries for height %d: %w", h, err)
		}

		if !executed {
			// if the finalized block has not been executed, then we finish indexing,
			// as we have finished indexing all executed blocks up to this point.
			// in happy case, all finalized blocks should have been executed.
			// this might happen when the execution node is catching up or during HCU.
			return nil
		}

		// Store registers directly to disk store
		err = b.registerStore.SaveRegisters(header, registerEntries)
		if err != nil {
			return fmt.Errorf("failed to store registers for height %d: %w", h, err)
		}

		// Throttle indexing rate if configured
		if b.heightsPerSecond > 0 && h < latestFinalized.Height {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sleepDuration):
				// Continue to next iteration
			}
		}
	}

	return nil
}
