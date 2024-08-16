package pruner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
)

var _ Core = (*PruneByViewCore)(nil)

// PruneByViewCore is a pruner that prunes blocks based on the height of the block.
type PruneByViewCore struct {
	*coreBase

	log      zerolog.Logger
	db       *pebble.DB
	headers  storage.Headers
	progress storage.ConsumerProgress

	lowestView       uint64
	pruningThreshold uint64

	inProgress *atomic.Bool
}

// NewPruneByViewCore creates a new PruneByViewCore.
func NewPruneByViewCore(
	log zerolog.Logger,
	db *pebble.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	progress storage.ConsumerProgress,
	lowestView uint64,
	pruningThreshold uint64,
) *PruneByViewCore {
	return &PruneByViewCore{
		coreBase:         newCoreBase(chunkDataPacks, DefaultThrottleDelay),
		log:              log.With().Str("component", "protocoldb_core_pruner").Logger(),
		db:               db,
		headers:          headers,
		progress:         progress,
		lowestView:       lowestView,
		pruningThreshold: pruningThreshold,
		inProgress:       atomic.NewBool(false),
	}
}

func (p *PruneByViewCore) Prune(ctx context.Context, baseHeader *flow.Header) error {
	if !p.inProgress.CompareAndSwap(false, true) {
		p.log.Debug().Uint64("base_view", baseHeader.View).Msg("pruning already in progress")
		return nil
	}
	defer p.inProgress.Store(false)

	if baseHeader.View < p.lowestView {
		return errors.New("inconsistent state: base view must not be lower than the lowest view")
	}
	if baseHeader.View-p.lowestView < p.pruningThreshold {
		return nil
	}

	start := time.Now()
	startView := p.lowestView
	endView := baseHeader.View

	lg := p.log.With().
		Uint64("start_view", p.lowestView).
		Uint64("end_view", endView).
		Logger()

	lg.Info().Msg("pruning views")

	err := p.prune(ctx, endView)
	if err != nil {
		lg.Err(err).
			Dur("duration_ms", time.Since(start)).
			Msg("pruning failed")

		return err
	}

	lg.Info().
		Dur("duration_ms", time.Since(start)).
		Uint64("blocks_pruned", endView-startView).
		Msg("pruning complete")

	return nil
}

func (p *PruneByViewCore) prune(ctx context.Context, endView uint64) error {
	batch := pebbleStorage.NewBatch(p.db)
	defer func() {
		if err := batch.Close(); err != nil {
			p.log.Error().Err(err).Msg("failed to close batch")
		}
	}()

	for view := p.lowestView; view < endView; view++ {
		if ctx.Err() != nil {
			return nil // abort the pruning operation and discard all uncommitted changes
		}

		if err := p.pruneView(view, batch); err != nil {
			return fmt.Errorf("could not prune to view: %w", err)
		}
	}

	if err := batch.Flush(); err != nil {
		return fmt.Errorf("could not commit batch: %w", err)
	}

	p.lowestView = endView

	// update the last pruned view only after persisting all changes
	if err := p.progress.SetProcessedIndex(p.lowestView - 1); err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}

// pruneView removes all data for the block with the given view
// forks are ignored since all blocks contained within them will eventually be pruned by view.
// No errors are expected during normal operation.
func (p *PruneByViewCore) pruneView(view uint64, batch storage.BatchStorage) error {
	header, err := p.headers.ByView(view)
	if err != nil {
		// missing views either means there were no valid blocks proposed for the view, or that the
		// block was already pruned, but the progress's processed index was not updated in the db
		// due to a crash
		if errors.Is(err, storage.ErrNotFound) {
			// skip views with no proposed blocks
			return nil
		}
		return fmt.Errorf("could not get header for view %d: %w", view, err)
	}
	headerID := header.ID()

	if err := p.pruneBlock(headerID, batch); err != nil {
		return fmt.Errorf("could not prune block (id: %s): %w", headerID, err)
	}

	return nil
}
