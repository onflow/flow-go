package pruner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
)

type PruneMethod int

const (
	// PruneByHeight prunes data using finalized block heights
	// This is should be used when a db does not include a view index
	PruneByHeight PruneMethod = iota + 1

	// PruneByView prunes data using views
	// This is the simpler and more efficient method, but requires the node to have a view index
	PruneByView

	// DefaultPruneInterval is the default interval between pruning cycles
	DefaultPruneInterval = 5 * time.Minute

	// DefaultThrottleDelay is the default delay between pruning individual blocks
	DefaultThrottleDelay = 20 * time.Millisecond
)

type Pruner struct {
	component.Component

	log zerolog.Logger
	db  *pebble.DB

	headers        storage.Headers
	chunkDataPacks storage.ChunkDataPacks

	targetKeepBlocks uint64 // number of blocks to keep below the highest block
	pruneThreshold   uint64 // number of blocks to exceed the target before pruning
	pruneMethod      PruneMethod

	lowestHeader     *flow.Header
	rootSealedHeader *flow.Header
	progress         storage.ConsumerProgress

	producers []*BlockProcessedProducer
	mu        sync.RWMutex
}

func New(
	log zerolog.Logger,
	db *pebble.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	rootSealedHeader *flow.Header,
	progress storage.ConsumerProgress,
	targetKeepBlocks uint64,
	pruneThreshold uint64,
) *Pruner {
	pruneMethod := PruneByHeight
	if progress.Consumer() == module.ConsumeProgressProtocolDBPrunerByView {
		pruneMethod = PruneByView
	}

	p := &Pruner{
		log: log.With().Str("component", "protocoldb_pruner").Logger(),
		db:  db,

		headers:          headers,
		chunkDataPacks:   chunkDataPacks,
		targetKeepBlocks: targetKeepBlocks,
		pruneThreshold:   pruneThreshold,
		rootSealedHeader: rootSealedHeader,

		progress:    progress,
		pruneMethod: pruneMethod,
	}

	p.Component = component.NewComponentManagerBuilder().
		AddWorker(p.loop).
		Build()

	return p
}

// Register registers an external component as a producer of BlockProcessed events.
// The pruner will take the lowest processed height from the set of producers, and use that as the
// base height for pruning.
func (p *Pruner) Register() *BlockProcessedProducer {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Note: the initial height is 0 so pruning will not start until all producers have completed
	// processing at least one block
	producer := NewBlockProcessedProducer()
	p.producers = append(p.producers, producer)

	return producer
}

// baseHeight returns the highest height that all block processors have completed processing
// for instance, if consensus is at 1000, and execution is at 750, the base height should be 750
// this ensures all subsystems have the data they need regardless of the pruning thresholds used
func (p *Pruner) baseHeight() uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	lowest := uint64(0)
	for _, producer := range p.producers {
		height := producer.highestCompleteHeight.Load()
		if height < lowest {
			lowest = height
		}
	}

	return lowest
}

// initProgress initializes the pruned height consumer progress
// If the progress is not found, it is initialized to the root sealed block height
// No errors are expected during normal operation.
func (p *Pruner) initProgress() (uint64, error) {
	lowestHeight, err := p.progress.ProcessedIndex()
	if err == nil {
		return lowestHeight, nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return 0, fmt.Errorf("could not get processed index: %w", err)
	}

	err = p.progress.InitProcessedIndex(p.rootSealedHeader.Height)
	if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
		return 0, fmt.Errorf("could not init processed index: %w", err)
	}

	p.log.Warn().
		Uint64("processed index", lowestHeight).
		Msg("processed index not found, initialized.")

	return p.progress.ProcessedIndex()
}

func (p *Pruner) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	lowestHeight, err := p.initProgress()
	if err != nil {
		ctx.Throw(err)
		return
	}

	p.lowestHeader, err = p.headers.ByHeight(lowestHeight)
	if err != nil {
		ctx.Throw(fmt.Errorf("could not get header for lowest height (%d): %w", lowestHeight, err))
		return
	}

	ready()

	ticker := time.NewTicker(DefaultPruneInterval)
	defer ticker.Stop()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// baseHeight is the highest height for which we have completed all processing
		baseHeight := p.baseHeight()
		if baseHeight < p.lowestHeader.Height {
			// this should never happen and means there's a bug. the lowest fully processed height
			// cannot be lower than the lowest height in the database
			ctx.Throw(errors.New("inconsistent state: base height must not be lower than the lowest height"))
			return
		}

		lg := p.log.With().Uint64("baseHeight", baseHeight).Logger()

		// if there are not enough blocks to prune, skip
		if baseHeight-p.lowestHeader.Height < p.pruneThreshold {
			lg.Debug().
				Uint64("lowestHeight", p.lowestHeader.Height).
				Msg("not enough blocks to prune. skipping pruning")
			continue
		}

		baseHeader, err := p.headers.ByHeight(baseHeight)
		if err != nil {
			ctx.Throw(fmt.Errorf("could not get header for height %d: %w", baseHeight, err))
			return
		}

		err = p.prune(ctx, baseHeader)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			ctx.Throw(err)
			return
		}

		// finally, increment the lowest height
		p.lowestHeader = baseHeader

		lg.Info().Msg("pruning complete")
	}
}

// prune removes data for all blocks up to the given header exclusive.
// No errors are expected during normal operation.
func (p *Pruner) prune(ctx context.Context, baseHeader *flow.Header) error {
	batch := pebbleStorage.NewBatch(p.db)
	defer func() {
		if err := batch.Close(); err != nil {
			p.log.Error().Err(err).Msg("failed to close batch")
		}
	}()

	if p.pruneMethod == PruneByView {
		if err := p.pruneToView(ctx, baseHeader.View, batch); err != nil {
			return fmt.Errorf("could not prune to view: %w", err)
		}
	} else {
		if err := p.pruneToHeight(ctx, baseHeader.Height, batch); err != nil {
			return fmt.Errorf("could not prune to height: %w", err)
		}
	}

	if err := batch.Flush(); err != nil {
		return fmt.Errorf("could not commit batch: %w", err)
	}

	return nil
}

// pruneBlock removes all data associated with a block
// this method contains all the logic to perform the actual pruning.
// No errors are expected during normal operation.
func (p *Pruner) pruneBlock(blockID flow.Identifier, batch storage.BatchStorage) error {
	time.Sleep(DefaultThrottleDelay)

	err := p.chunkDataPacks.Prune(blockID, batch)
	if err != nil {
		return fmt.Errorf("could not prune chunk data packs: %w", err)
	}

	return nil
}

// pruneToView removes all data for all blocks up to but excluding the given view
// forks are ignored since all blocks contained within them will eventually be pruned by view.
// No errors are expected during normal operation.
func (p *Pruner) pruneToView(ctx context.Context, lastView uint64, batch storage.BatchStorage) error {
	currentView := p.lowestHeader.View
	for currentView < lastView {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		header, err := p.headers.ByView(currentView)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// skip views with no proposed blocks
				currentView++
				continue
			}
			return fmt.Errorf("could not get header for view %d: %w", currentView, err)
		}
		headerID := header.ID()

		if err := p.pruneBlock(headerID, batch); err != nil {
			return fmt.Errorf("could not prune block (id: %s): %w", headerID, err)
		}

		// store the recently pruned view as the processed index
		if err := p.progress.SetProcessedIndex(currentView); err != nil {
			return fmt.Errorf("could not update processed index: %w", err)
		}

		// finally, increment the lowest height
		currentView++
	}

	return nil
}

// pruneToHeight removes all data for all blocks up to but excluding the given height, including all forks
// No errors are expected during normal operation.
func (p *Pruner) pruneToHeight(ctx context.Context, end uint64, batch storage.BatchStorage) error {
	lowestHeight := p.lowestHeader.Height
	for lowestHeight < end {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// starting from the lowest height,
		finalizedHeader, err := p.headers.ByHeight(lowestHeight)
		if err != nil {
			return fmt.Errorf("could not get lowest header: %w", err)
		}
		finalizedHeaderID := finalizedHeader.ID()

		// find all peer forks
		siblings, err := p.headers.ByParentID(finalizedHeader.ParentID)
		if err != nil {
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

		// store the recently pruned height as the processed index
		if err := p.progress.SetProcessedIndex(lowestHeight); err != nil {
			return fmt.Errorf("could not update processed index: %w", err)
		}

		// finally, increment the lowest height
		lowestHeight++
	}

	return nil
}

// pruneAllFromFork recursively removes all blocks in the tree descending from the given block, excluding the block itself
// No errors are expected during normal operation.
func (p *Pruner) pruneAllFromFork(ancestorBlockID flow.Identifier, batch storage.BatchStorage) error {
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
