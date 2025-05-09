package cache

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
)

var (
	ErrDisconnectedBatch = errors.New("batch must be a sequence of connected blocks")
)

type BlocksByID map[flow.Identifier]*flow.BlockProposal

// batchContext contains contextual data for batch of blocks. Per convention, a batch is
// a continuous sequence of blocks, i.e. `batch[k]` is the parent block of `batch[k+1]`.
type batchContext struct {
	batchParent *flow.BlockProposal // immediate parent of the first block in batch, i.e. `batch[0]`
	batchChild  *flow.BlockProposal // immediate child of the last block in batch, i.e. `batch[len(batch)-1]`

	// equivocatingBlocks holds the list of equivocations that the batch contained, when comparing to the
	// cached blocks. An equivocation are two blocks for the same view that have different block IDs.
	equivocatingBlocks [][2]*flow.BlockProposal

	// redundant marks if ALL blocks in batch are already stored in cache, meaning that
	// such input is identical to what was previously processed.
	redundant bool
}

// Cache stores pending blocks received from other replicas, caches blocks by blockID, and maintains
// secondary indices to look up blocks by view or by parent ID. Additional indices are used to track proposal equivocation
// (multiple valid proposals for same block) and find blocks not only by parent but also by child.
// Resolves certified blocks when processing incoming batches.
// Concurrency safe.
type Cache struct {
	backend *herocache.Cache[*flow.BlockProposal] // cache with random ejection
	lock    sync.RWMutex

	// secondary indices
	byView   map[uint64]BlocksByID          // lookup of blocks by their respective view; used to detect equivocation
	byParent map[flow.Identifier]BlocksByID // lookup of blocks by their parentID, for finding a block's known children

	notifier   hotstuff.ProposalViolationConsumer // equivocations will be reported using this notifier
	lowestView counters.StrictMonotonicCounter    // lowest view that the cache accepts blocks for
}

// Peek performs lookup of cached block by blockID.
// Concurrency safe
func (c *Cache) Peek(blockID flow.Identifier) *flow.BlockProposal {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if block, found := c.backend.Get(blockID); found {
		return block
	} else {
		return nil
	}
}

// NewCache creates new instance of Cache
func NewCache(log zerolog.Logger, limit uint32, collector module.HeroCacheMetrics, notifier hotstuff.ProposalViolationConsumer) *Cache {
	// We consume ejection event from HeroCache to here to drop ejected blocks from our secondary indices.
	distributor := NewDistributor[*flow.BlockProposal]()
	cache := &Cache{
		backend: herocache.NewCache[*flow.BlockProposal](
			limit,
			herocache.DefaultOversizeFactor,
			heropool.RandomEjection,
			log.With().Str("component", "follower.cache").Logger(),
			collector,
			herocache.WithTracer[*flow.BlockProposal](distributor),
		),
		byView:   make(map[uint64]BlocksByID),
		byParent: make(map[flow.Identifier]BlocksByID),
		notifier: notifier,
	}
	distributor.AddConsumer(cache.handleEjectedBlock)
	return cache
}

// handleEjectedBlock performs cleanup of secondary indexes to prevent memory leaks.
// WARNING: Concurrency safety of this function is guaranteed by `c.lock`. This method is only called
// by `herocache.Cache.Add` and we perform this call while `c.lock` is in locked state.
func (c *Cache) handleEjectedBlock(proposal *flow.BlockProposal) {
	blockID := proposal.Block.ID()

	// remove block from the set of blocks for this view
	blocksForView := c.byView[proposal.Block.Header.View]
	delete(blocksForView, blockID)
	if len(blocksForView) == 0 {
		delete(c.byView, proposal.Block.Header.View)
	}

	// remove block from the parent's set of its children
	siblings := c.byParent[proposal.Block.Header.ParentID]
	delete(siblings, blockID)
	if len(siblings) == 0 {
		delete(c.byParent, proposal.Block.Header.ParentID)
	}
}

// AddBlocks atomically adds the given batch of blocks to the cache.
// We require that incoming batch is sorted in ascending height order and doesn't have skipped blocks;
// otherwise the cache returns a `ErrDisconnectedBatch` error. When receiving batch: [first, ..., last],
// we are only interested in the first and last blocks. All blocks before `last` are certified by
// construction (by the QC included in `last`). The following two cases are possible:
// - for first block:
//   - no parent available for first block.
//   - parent for first block available in cache allowing to certify it, we can certify one extra block(parent).
//
// - for last block:
//   - no child available for last block, need to wait for child to certify it (certify one fewer block).
//   - child for last block available in cache allowing to certify it, we can certify the last block.
//
// All blocks from the batch are stored in the cache to provide deduplication.
// The function returns any new certified chain of blocks created by addition of the batch.
// Returns `certifiedBatch` if the input batch has more than one block, and/or if either a child
// or parent of the batch is in the cache. The implementation correctly handles cases with `len(batch) == 1`
// or `len(batch) == 0`, where it returns `nil` in the following cases:
//   - the input batch has exactly one block and neither its parent nor child is in the cache.
//   - the input batch is empty
//
// If message equivocation was detected it will be reported using a notification.
// Concurrency safe.
//
// Expected errors during normal operations:
//   - ErrDisconnectedBatch
func (c *Cache) AddBlocks(batch []*flow.BlockProposal) (certifiedBatch []flow.CertifiedBlock, err error) {
	batch = c.trimLeadingBlocksBelowPruningThreshold(batch)

	if len(batch) < 1 { // empty batch is no-op
		return nil, nil
	}

	// precompute block IDs (outside of lock) and sanity-check batch itself that blocks are connected
	blockIDs, err := enforceSequentialBlocks(batch)
	if err != nil {
		return nil, err
	}

	// Single atomic operation (main logic), with result returned as `batchContext`
	//  * add the given batch of blocks to the cache
	//  * check for equivocating blocks (result stored in `batchContext.equivocatingBlocks`)
	//  * check whether first block in batch (index 0) has a parent already in the cache
	//    (result stored in `batchContext.batchParent`)
	//  * check whether last block in batch has a child already in the cache
	//    (result stored in `batchContext.batchChild`)
	//  * check if input is redundant (indicated by `batchContext.redundant`), i.e. ALL blocks
	//    are already known: then skip further processing
	bc := c.unsafeAtomicAdd(blockIDs, batch)
	if bc.redundant {
		return nil, nil
	}

	// If there exists a parent for the batch's first block, then this is parent is certified
	// by the batch. Hence, we prepend certifiedBatch by the parent.
	if bc.batchParent != nil {
		batch = append([]*flow.BlockProposal{bc.batchParent}, batch...)
	}
	// If a child of the last block in the batch already exists in the cache: Then the entire batch is certified,
	// and we append the child to the batch. Hence, after this operation the following holds: all blocks in the
	// batch _except_ for the last one, their certifying QC is in the subsequent batch element.
	if bc.batchChild != nil {
		batch = append(batch, bc.batchChild)
	}

	certifiedBatch = make([]flow.CertifiedBlock, 0, len(batch)-1)
	for i, proposal := range batch[:len(batch)-1] {
		certifiedBlock, err := flow.NewCertifiedBlock(proposal, batch[i+1].Block.ToHeader().QuorumCertificate())
		if err != nil {
			return nil, fmt.Errorf("could not construct certified block: %w", err)
		}
		certifiedBatch = append(certifiedBatch, certifiedBlock)
	}
	// caution: in the case `len(batch) == 1`, the `certifiedBatch` might be empty now (if there was no batchParent or batchChild)

	// report equivocations
	for _, pair := range bc.equivocatingBlocks {
		c.notifier.OnDoubleProposeDetected(model.BlockFromFlow(pair[0].Block.ToHeader()), model.BlockFromFlow(pair[1].Block.ToHeader()))
	}

	if len(certifiedBatch) < 1 {
		return nil, nil
	}

	return certifiedBatch, nil
}

// PruneUpToView sets the lowest view that we are accepting blocks for. Any blocks
// with view _strictly smaller_ that the given threshold are removed from the cache.
// Concurrency safe.
func (c *Cache) PruneUpToView(view uint64) {
	previousPruningThreshold := c.lowestView.Value()
	if previousPruningThreshold >= view {
		return // removing all entries up to view was already done in an earlier call
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	if !c.lowestView.Set(view) {
		return // some other concurrent call to `PruneUpToView` did the work already
	}
	if len(c.byView) == 0 {
		return // empty, noting to prune
	}

	// Optimization: if there are less elements in the `byView` map
	// than the view range to prune: inspect each map element.
	// Otherwise, go through each view to prune.
	if uint64(len(c.byView)) < view-previousPruningThreshold {
		for v, blocks := range c.byView {
			if v < view {
				c.removeByView(v, blocks)
			}
		}
	} else {
		for v := previousPruningThreshold; v < view; v++ {
			if blocks, found := c.byView[v]; found {
				c.removeByView(v, blocks)
			}
		}
	}
}

// removeByView removes all blocks for the given view.
// NOT concurrency safe: execute within Cache's lock.
func (c *Cache) removeByView(view uint64, blocks BlocksByID) {
	for blockID, block := range blocks {
		c.backend.Remove(blockID)

		siblings := c.byParent[block.Block.Header.ParentID]
		delete(siblings, blockID)
		if len(siblings) == 0 {
			delete(c.byParent, block.Block.Header.ParentID)
		}
	}

	delete(c.byView, view)
}

// unsafeAtomicAdd does the following within a single atomic operation:
//   - add the given batch of blocks to the cache
//   - check for equivocating blocks
//   - check whether first block in batch (index 0) has a parent already in the cache
//   - check whether last block in batch has a child already in the cache
//   - check whether all blocks were previously stored in the cache
//
// Concurrency SAFE.
//
// For internal use only and unsafe in the following aspects
//   - assumes batch is _not empty_
//   - batch must form a sequence of sequential blocks, i.e. `batch[k]` is parent of `batch[k+1]`
//   - requires pre-computed blockIDs in the same order as fullBlocks
//
// Any errors are symptoms of internal state corruption.
func (c *Cache) unsafeAtomicAdd(blockIDs []flow.Identifier, fullBlocks []*flow.BlockProposal) (bc batchContext) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// check whether we have the parent of first block already in our cache:
	if parent, ok := c.backend.Get(fullBlocks[0].Block.Header.ParentID); ok {
		bc.batchParent = parent
	}

	// check whether we have a child of last block already in our cache:
	lastBlockID := blockIDs[len(blockIDs)-1]
	if children, ok := c.byParent[lastBlockID]; ok {
		// Due to forks, it is possible that we have multiple children for same parent. Conceptually we only
		// care for the QC that is contained in the child, which serves as proof that the parent has been
		// certified. Therefore, we don't care which child we find here, as long as we find one at all.
		for _, child := range children {
			bc.batchChild = child
			break
		}
	}

	// add blocks to underlying cache, check for equivocation and report if detected
	storedBlocks := uint64(0)
	for i, block := range fullBlocks {
		equivocation, cached := c.cache(blockIDs[i], block)
		if equivocation != nil {
			bc.equivocatingBlocks = append(bc.equivocatingBlocks, [2]*flow.BlockProposal{equivocation, block})
		}
		if cached {
			storedBlocks++
		}
	}
	bc.redundant = storedBlocks < 1

	return bc
}

// cache adds the given block to the underlying block cache. By indexing blocks by view, we can detect
// equivocation. The first return value contains the already-cached equivocating block or `nil` otherwise.
// Repeated calls with the same block are no-ops.
// CAUTION: not concurrency safe: execute within Cache's lock.
func (c *Cache) cache(blockID flow.Identifier, block *flow.BlockProposal) (equivocation *flow.BlockProposal, stored bool) {
	cachedBlocksAtView, haveCachedBlocksAtView := c.byView[block.Block.Header.View]
	// Check whether there is a block with the same view already in the cache.
	// During happy-path operations `cachedBlocksAtView` contains usually zero blocks or exactly one block, which
	// is our input `block` (duplicate). Larger sets of blocks can only be caused by slashable byzantine actions.
	for otherBlockID, otherBlock := range cachedBlocksAtView {
		if otherBlockID == blockID {
			return nil, false // already stored
		}
		// have two blocks for the same view but with different IDs => equivocation!
		equivocation = otherBlock
		break // we care whether we find an equivocation, but don't need to enumerate all equivocations
	}
	// Note: Even if this node detects an equivocation, we still have to process the block. This is because
	// the node might be the only one seeing the equivocation, and other nodes might certify the block,
	// in which case also this node needs to process the block to continue following consensus.

	// block is not a duplicate: store in the underlying HeroCache and add it to secondary indices
	stored = c.backend.Add(blockID, block)
	if !stored { // future proofing code: we allow an overflowing HeroCache to potentially eject the newly added element.
		return
	}

	// populate `byView` index
	if !haveCachedBlocksAtView {
		cachedBlocksAtView = make(BlocksByID)
		c.byView[block.Block.Header.View] = cachedBlocksAtView
	}
	cachedBlocksAtView[blockID] = block

	// populate `byParent` index
	siblings, ok := c.byParent[block.Block.Header.ParentID]
	if !ok {
		siblings = make(BlocksByID)
		c.byParent[block.Block.Header.ParentID] = siblings
	}
	siblings[blockID] = block

	return
}

// enforceSequentialBlocks enforces that batch is a continuous sequence of blocks, i.e. `batch[k]`
// is the parent block of `batch[k+1]`. Returns a slice with IDs of the blocks in the same order
// as batch. Returns `ErrDisconnectedBatch` if blocks are not a continuous sequence.
// Pure function, hence concurrency safe.
func enforceSequentialBlocks(batch []*flow.BlockProposal) ([]flow.Identifier, error) {
	blockIDs := make([]flow.Identifier, 0, len(batch))
	parentID := batch[0].Block.ID()
	blockIDs = append(blockIDs, parentID)
	for _, b := range batch[1:] {
		if b.Block.Header.ParentID != parentID {
			return nil, ErrDisconnectedBatch
		}
		parentID = b.Block.ID()
		blockIDs = append(blockIDs, parentID)
	}
	return blockIDs, nil
}

// trimLeadingFinalizedBlocks trims the blocks at the _beginning_ of the batch, whose views
// are smaller or equal to the lowest pruned view. Formally, let i be the _smallest_ index such that
//
//	batch[i].View ≥ lowestView
//
// Hence, for all k < i: batch[k].View < lowestView (otherwise, a smaller value for i exists).
// Note:
//   - For this method, we do _not_ assume any specific ordering of the blocks.
//   - We drop all blocks at the _beginning_ that we anyway would not want to cache.
//   - The returned slice of blocks could still contain blocks with views below the cutoff.
func (c *Cache) trimLeadingBlocksBelowPruningThreshold(batch []*flow.BlockProposal) []*flow.BlockProposal {
	lowestView := c.lowestView.Value()
	for i, block := range batch {
		if block.Block.Header.View >= lowestView {
			return batch[i:]
		}
	}
	return nil
}
