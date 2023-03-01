package cache

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
)

// OnEquivocation is a callback to report observing two different blocks with the same view.
type OnEquivocation func(first *flow.Block, other *flow.Block)
type BlocksByID map[flow.Identifier]*flow.Block

// Cache stores pending blocks received from other replicas, caches blocks by blockID, it also
// maintains secondary index by view and by parent. Additional indexes are used to track proposal equivocation
// (multiple valid proposals for same block) and find blocks not only by parent but also by child.
// Resolves certified blocks when processing incoming batches.
// Concurrency safe.
type Cache struct {
	backend *herocache.Cache // cache with random ejection
	lock    sync.RWMutex
	// secondary index by view, can be used to detect equivocation
	byView map[uint64]*flow.Block
	// secondary index by parentID, can be used to find child of the block
	byParent map[flow.Identifier]BlocksByID
	// when message equivocation has been detected report it using this callback
	onEquivocation OnEquivocation
}

// Peek performs lookup of cached block by blockID.
// Concurrency safe
func (c *Cache) Peek(blockID flow.Identifier) *flow.Block {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if block, found := c.backend.ByID(blockID); found {
		return block.(*flow.Block)
	} else {
		return nil
	}
}

// NewCache creates new instance of Cache, as part of construction process connects ejection event from HeroCache to
// post-ejection processing logic to perform cleanup of secondary indexes to prevent memory leaks.
func NewCache(log zerolog.Logger, limit uint32, collector module.HeroCacheMetrics, onEquivocation OnEquivocation) *Cache {
	distributor := NewDistributor(collector)
	cache := &Cache{
		backend: herocache.NewCache(
			limit,
			herocache.DefaultOversizeFactor,
			heropool.RandomEjection,
			log.With().Str("component", "follower.cache").Logger(),
			distributor,
		),
		byView:         make(map[uint64]*flow.Block),
		byParent:       make(map[flow.Identifier]BlocksByID),
		onEquivocation: onEquivocation,
	}
	distributor.AddConsumer(cache.handleEjectedEntity)
	return cache
}

// handleEjectedEntity performs cleanup of secondary indexes to prevent memory leaks.
// WARNING: Concurrency safety of this function is guaranteed by s.lock, this callback can be called
// only in herocache.Cache.Add and we perform this call while s.lock is in locked state.
func (c *Cache) handleEjectedEntity(entity flow.Entity) {
	block := entity.(*flow.Block)
	delete(c.byView, block.Header.View)
	blocksByID := c.byParent[block.Header.ParentID]
	delete(blocksByID, block.ID())
	if len(blocksByID) == 0 {
		delete(c.byParent, block.Header.ParentID)
	}
}

// AddBlocks atomically applies batch of blocks to the cache of pending but not yet certified blocks. Upon insertion cache tries to resolve
// incoming blocks to what is stored in the cache.
// We require that incoming batch is sorted in ascending height order and doesn't have skipped blocks.
// When receiving batch: [first, ..., last], we are only interested in first and last blocks. All blocks before
// `last` are certified by construction (by the QC included in `last`).
// Next scenarios are possible:
// - for first block:
//   - no parent available for first block.
//   - parent for first block available in cache allowing to certify it, we can certify one extra block(parent).
//
// - for last block:
//   - no child available for last block, need to wait for child to certify it.
//   - child for last block available in cache allowing to certify it, we can certify one extra block(child).
//
// All blocks from the batch are stored in the cache to provide deduplication.
// The function returns any new certified chain of blocks created by addition of the batch.
// Returns `nil, nil` if the input batch has exactly one block and neither its parent nor child is in the cache.
// Returns `certifiedBatch, certifyingQC` if the input batch has more than one block, and/or if either a child
// or parent of the batch is in the cache.
// Note that implementation behaves correctly where len(batch) == 1.
// If message equivocation was detected it will be reported using a notification.
// Concurrency safe.
func (c *Cache) AddBlocks(batch []*flow.Block) (certifiedBatch []*flow.Block, certifyingQC *flow.QuorumCertificate) {
	var equivocatedBlocks [][]*flow.Block

	// prefill certifiedBatch with minimum viable result
	// since batch is a chain of blocks, then by definition all except the last one
	// has to be certified by definition
	certifiedBatch = batch[:len(batch)-1]

	if len(batch) > 1 {
		// set certifyingQC, QC from last block certifies complete batch
		certifyingQC = batch[len(batch)-1].Header.QuorumCertificate()
	}
	lastBlockID := batch[len(batch)-1].ID()

	c.lock.Lock()
	// check for message equivocation, report any if detected
	for _, block := range batch {
		if otherBlock, ok := c.byView[block.Header.View]; ok {
			if otherBlock.ID() != block.ID() {
				equivocatedBlocks = append(equivocatedBlocks, []*flow.Block{otherBlock, block})
			}
		} else {
			c.byView[block.Header.View] = block
		}
		blockID := block.ID()
		// store all blocks in the cache to provide deduplication
		c.backend.Add(blockID, block)
		blocksByID, ok := c.byParent[block.Header.ParentID]
		if !ok {
			blocksByID = make(BlocksByID)
			c.byParent[block.Header.ParentID] = blocksByID
		}
		blocksByID[blockID] = block
	}

	firstBlock := batch[0]           // lowest height/view
	lastBlock := batch[len(batch)-1] // highest height/view

	// start by checking if batch certifies any block that was stored in the cache
	if parent, ok := c.backend.ByID(firstBlock.Header.ParentID); ok {
		// parent found, it can be certified by the batch, we need to include it to the certified blocks
		certifiedBatch = append([]*flow.Block{parent.(*flow.Block)}, certifiedBatch...)
		// set certifyingQC, QC from last block certifies complete batch
		certifyingQC = batch[len(batch)-1].Header.QuorumCertificate()
	}

	// check if there is a block in cache that certifies last block of the batch.
	if children, ok := c.byParent[lastBlockID]; ok {
		// it's possible that we have multiple children for same parent, this situation is possible
		// when we had fork at some level. Conceptually we don't care what QC certifies block since QCs
		// form an equivalence class. Because of this we will take QC from first child that we know of.
		for _, child := range children {
			// child found in cache, meaning we can certify last block
			// no need to store anything since the block is certified and child is already in cache
			certifiedBatch = append(certifiedBatch, lastBlock)
			// in this case we will get a new certifying QC
			certifyingQC = child.Header.QuorumCertificate()

			break
		}
	}

	c.lock.Unlock()

	// report equivocation
	for _, pair := range equivocatedBlocks {
		c.onEquivocation(pair[0], pair[1])
	}
	return certifiedBatch, certifyingQC
}
