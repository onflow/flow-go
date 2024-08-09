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

var _ Core = (*PruneByHeightCore)(nil)

// PruneByHeightCore is a pruner that prunes blocks based on the height of the block.
type PruneByHeightCore struct {
	*coreBase

	log      zerolog.Logger
	db       *pebble.DB
	headers  storage.Headers
	progress storage.ConsumerProgress

	lowestHeight     uint64
	pruningThreshold uint64

	inProgress *atomic.Bool
}

// NewPruneByHeightCore creates a new PruneByHeightCore.
func NewPruneByHeightCore(
	log zerolog.Logger,
	db *pebble.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	progress storage.ConsumerProgress,
	lowestHeight uint64,
	pruningThreshold uint64,
) *PruneByHeightCore {
	return &PruneByHeightCore{
		coreBase:         newCoreBase(chunkDataPacks, DefaultThrottleDelay),
		log:              log,
		db:               db,
		headers:          headers,
		progress:         progress,
		lowestHeight:     lowestHeight,
		pruningThreshold: pruningThreshold,
		inProgress:       atomic.NewBool(false),
	}
}

func (p *PruneByHeightCore) Prune(ctx context.Context, baseHeader *flow.Header) error {
	if !p.inProgress.CompareAndSwap(false, true) {
		p.log.Debug().Uint64("base_height", baseHeader.Height).Msg("pruning already in progress")
		return nil
	}
	defer p.inProgress.Store(false)

	if baseHeader.Height < p.lowestHeight {
		return errors.New("inconsistent state: base height must not be lower than the lowest height")
	}
	if baseHeader.Height-p.lowestHeight < p.pruningThreshold {
		return nil
	}

	start := time.Now()
	startHeight := p.lowestHeight
	endHeight := baseHeader.Height

	lg := p.log.With().
		Uint64("start_height", startHeight).
		Uint64("end_height", endHeight).
		Logger()

	lg.Info().Msg("pruning heights")

	err := p.prune(ctx, endHeight)
	if err != nil {
		lg.Err(err).
			Dur("duration_ms", time.Since(start)).
			Msg("pruning failed")

		return err
	}

	lg.Info().
		Dur("duration_ms", time.Since(start)).
		Uint64("blocks_pruned", endHeight-startHeight).
		Msg("pruning complete")

	return nil
}

func (p *PruneByHeightCore) prune(ctx context.Context, endHeight uint64) error {
	batch := pebbleStorage.NewBatch(p.db)
	defer func() {
		if err := batch.Close(); err != nil {
			p.log.Error().Err(err).Msg("failed to close batch")
		}
	}()

	for height := p.lowestHeight; height < endHeight; height++ {
		if ctx.Err() != nil {
			return nil // abort the pruning operation and discard all uncommitted changes
		}

		if err := p.pruneHeight(height, batch); err != nil {
			return fmt.Errorf("could not prune to height: %w", err)
		}
	}

	if err := batch.Flush(); err != nil {
		return fmt.Errorf("could not commit batch: %w", err)
	}

	p.lowestHeight = endHeight

	// update the last pruned height only after persisting all changes
	if err := p.progress.SetProcessedIndex(p.lowestHeight - 1); err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}

// pruneHeight removes all data a blocks, including all forks with the same parent
// No errors are expected during normal operation.
func (p *PruneByHeightCore) pruneHeight(height uint64, batch storage.BatchStorage) error {
	finalizedHeader, err := p.headers.ByHeight(height)
	if err != nil {
		// this most likely means that the block was pruned, but the progress's processed index was
		// not updated in the db due to a crash. If this is happening frequently, it may be a bug.
		if errors.Is(err, storage.ErrNotFound) {
			p.log.Info().Uint64("height", height).Msg("no block found at height")
			return nil
		}
		return fmt.Errorf("could not get lowest header: %w", err)
	}
	finalizedHeaderID := finalizedHeader.ID()

	// find all peer forks
	siblings, err := p.headers.ByParentID(finalizedHeader.ParentID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			p.log.Warn().Str("sibling_id", finalizedHeader.ParentID.String()).Msg("sibling not found")
			return nil
		}

		// this should never return NotFound since finalizedHeader should be included in the list
		return fmt.Errorf("could not get siblings: %w", err)
	}

	// prune all peer forks, and the lowest block
	for _, sibling := range siblings {
		siblingID := sibling.ID()
		if siblingID != finalizedHeaderID {
			if err := p.pruneAllFromFork(siblingID, batch); err != nil {

				return fmt.Errorf("could not prune sibling fork: %w", err)
			}
		}
		if err := p.pruneBlock(siblingID, batch); err != nil {
			return fmt.Errorf("could not prune block (id: %s): %w", siblingID, err)
		}
	}

	return nil
}

// pruneAllFromFork recursively removes all blocks in the tree descending from the given block, excluding the block itself
// No errors are expected during normal operation.
func (p *PruneByHeightCore) pruneAllFromFork(ancestorBlockID flow.Identifier, batch storage.BatchStorage) error {
	// get all children of the ancestor block
	children, err := p.headers.ByParentID(ancestorBlockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// stop searching when the last block in the chain is reached
			// i.e. the first block with no children
			return nil
		}
		return fmt.Errorf("could not get children (parent: %s): %w", ancestorBlockID, err)
	}

	// prune all blocks in the tree in a depth-first manner
	for _, child := range children {
		childID := child.ID()
		// recursively search all child forks until we reach the leaves
		if err = p.pruneAllFromFork(childID, batch); err != nil {
			return fmt.Errorf("could not remove child (%s): %w", childID, err)
		}

		// prune the block
		if err = p.pruneBlock(childID, batch); err != nil {
			return fmt.Errorf("could not prune child (%s): %w", childID, err)
		}
	}

	return nil
}
