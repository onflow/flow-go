package queue

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
)

// ChunkDataPackRequestQueue implements a HeroCache-based in-memory queue for storing chunk data pack requests.
// It is designed to be utilized at Execution Nodes to maintain and respond chunk data pack requests.
type ChunkDataPackRequestQueue struct {
	mu        sync.RWMutex
	cache     *herocache.Cache
	sizeLimit uint
}

var _ mempool.ChunkDataPackRequestQueue = &ChunkDataPackRequestQueue{}

func NewChunkDataPackRequestQueue(sizeLimit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *ChunkDataPackRequestQueue {
	return &ChunkDataPackRequestQueue{
		cache: herocache.NewCache(
			sizeLimit,
			herocache.DefaultOversizeFactor,
			heropool.LRUEjection,
			logger.With().Str("mempool", "chunk-data-pack-request-queue-herocache").Logger(),
			collector),
		sizeLimit: uint(sizeLimit),
	}
}

// Push stores chunk data pack request into the queue.
// Boolean returned variable determines whether push was successful, i.e.,
// push may be dropped if queue is full or already exists.
func (c *ChunkDataPackRequestQueue) Push(chunkId flow.Identifier, requesterId flow.Identifier) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache.Size() >= c.sizeLimit {
		return false
	}

	req := chunkDataPackRequestEntity{
		ChunkDataPackRequest: mempool.ChunkDataPackRequest{
			ChunkId:     chunkId,
			RequesterId: requesterId,
		},
		id: identifierOf(chunkId, requesterId),
	}

	return c.cache.Add(req.id, req)
}

// Head returns the head of queue.
// Boolean return value determines whether there is a head available.
func (c *ChunkDataPackRequestQueue) Head() (*mempool.ChunkDataPackRequest, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	head, ok := c.cache.Head()
	if !ok {
		// cache is empty, and there is no head.
		return nil, false
	}

	request := head.(chunkDataPackRequestEntity)
	return &mempool.ChunkDataPackRequest{RequesterId: request.RequesterId, ChunkId: request.ChunkId}, true
}

// Pop removes and returns the head of queue, and updates the head to the next element.
// Boolean return value determines whether pop is successful, i.e., poping an empty queue returns false.
func (c *ChunkDataPackRequestQueue) Pop() (*mempool.ChunkDataPackRequest, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	head, ok := c.cache.Head()
	if !ok {
		// cache is empty, and there is no head yet to pop.
		return nil, false
	}

	c.cache.Remove(head.ID())
	request := head.(chunkDataPackRequestEntity)
	return &mempool.ChunkDataPackRequest{RequesterId: request.RequesterId, ChunkId: request.ChunkId}, true
}

func (c *ChunkDataPackRequestQueue) Size() uint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cache.Size()
}

// chunkDataPackRequestEntity is a wrapper around ChunkDataPackRequest that implements Entity interface for it, and
// also internally caches its identifier.
type chunkDataPackRequestEntity struct {
	mempool.ChunkDataPackRequest
	// caching identifier to avoid cpu overhead
	// per query.
	id flow.Identifier
}

var _ flow.Entity = &chunkDataPackRequestEntity{}

func (c chunkDataPackRequestEntity) ID() flow.Identifier {
	return c.id
}

func (c chunkDataPackRequestEntity) Checksum() flow.Identifier {
	return c.id
}

func identifierOf(chunkId flow.Identifier, requesterId flow.Identifier) flow.Identifier {
	return flow.MakeID(append(chunkId[:], requesterId[:]...))
}
