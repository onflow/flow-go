package follower

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
)

type OnEquivocation func(first *flow.Block, other *flow.Block)

// Cache stores pending blocks received from other replicas, caches blocks by blockID it also
// maintains secondary index by view and by parent.
// Performs resolving of certified blocks when processing incoming batches.
// Concurrency safe.
type Cache struct {
	backend *herocache.Cache // cache with random ejection
	lock    sync.RWMutex
	// secondary index by view, can be used to detect equivocation
	byView map[uint64]*flow.Block
	// secondary index by parentID, can be used to find child of the block
	byParent map[flow.Identifier]*flow.Block
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

func NewCache(log zerolog.Logger, limit uint32, collector module.HeroCacheMetrics, onEquivocation OnEquivocation) *Cache {
	return &Cache{
		backend: herocache.NewCache(
			limit,
			herocache.DefaultOversizeFactor,
			heropool.RandomEjection,
			log.With().Str("follower", "cache").Logger(),
			collector,
		),
		byView:         make(map[uint64]*flow.Block, 0),
		byParent:       make(map[flow.Identifier]*flow.Block, 0),
		onEquivocation: onEquivocation,
	}
}

// AddBlocks atomically applies batch of blocks to the cache of pending but not yet certified blocks. Upon insertion cache tries to resolve
// incoming blocks to what is stored in the cache.
// When receiving batch: [first, ..., last], we are only interested in first and last blocks since all other blocks will be certified by definition.
// Next scenarios are possible:
// - for first block:
//   - no parent available for first block, we need to cache it since it will be used to certify parent when it's available.
//   - parent for first block available in cache allowing to certify it, no need to store first block in cache.
//
// - for last block:
//   - no child available for last block, we need to cache it since it's not certified yet.
//   - child for last block available in cache allowing to certify it, no need to store last block in cache.
//
// Note that implementation behaves correctly where len(batch) == 1.
// If message equivocation was detected it will be reported using a notification.
func (c *Cache) AddBlocks(batch []*flow.Block) (certifiedBatch []*flow.Block, certifyingQC *flow.QuorumCertificate) {
	var equivocatedBlocks [][]*flow.Block

	// prefill certifiedBatch with minimum viable result
	// since batch is a chain of blocks, then by definition all except the last one
	// has to be certified by definition
	certifiedBatch = batch[:len(batch)-1]

	c.lock.Lock()
	// check for message equivocation, report any if detected
	for _, block := range batch {
		if otherBlock, ok := c.byView[block.Header.View]; ok {
			equivocatedBlocks = append(equivocatedBlocks, []*flow.Block{otherBlock, block})
		} else {
			c.byView[block.Header.View] = block
		}
		// store all blocks in the cache to provide deduplication
		c.backend.Add(block.ID(), block)
		c.byParent[block.Header.ParentID] = block
	}

	firstBlock := batch[0]           // lowest height/view
	lastBlock := batch[len(batch)-1] // highest height/view

	// start by checking if batch certifies any block that was stored in the cache
	if parent, ok := c.backend.ByID(firstBlock.Header.ParentID); ok {
		// parent found, it can be certified by the batch, we need to include it to the certified blocks
		certifiedBatch = append([]*flow.Block{parent.(*flow.Block)}, certifiedBatch...)
		// set certifyingQC, QC from last block in batch certifies all batch
		certifyingQC = batch[len(batch)-1].Header.QuorumCertificate()
	}

	// check if there is a block in cache that certifies last block of the batch.
	if child, ok := c.byParent[lastBlock.ID()]; ok {
		// child found in cache, meaning we can certify last block
		// no need to store anything since the block is certified and child is already in cache
		certifiedBatch = append(certifiedBatch, lastBlock)
		// in this case we will get a new certifying QC
		certifyingQC = child.Header.QuorumCertificate()
	}

	c.lock.Unlock()

	// report equivocation
	for _, pair := range equivocatedBlocks {
		c.onEquivocation(pair[0], pair[1])
	}
	return certifiedBatch, certifyingQC
}
