package queue

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/rs/zerolog"
)

type ChunkDataPackRequestQueue struct {
	logger    zerolog.Logger
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

func (c *ChunkDataPackRequestQueue) Push(chunkId flow.Identifier, requesterId flow.Identifier) bool {
	lg := c.logger.With().
		Hex("chunk_id", logging.ID(chunkId)).
		Hex("requester_id", logging.ID(requesterId)).Logger()

	if c.cache.Size() > c.sizeLimit {
		lg.Debug().Msg("cannot push chunk data pack request to queue, queue is full")
		return false
	}

	req := chunkDataPackRequest{
		requesterId: requesterId,
		chunkId:     chunkId,
		id:          identifierOf(chunkId, requesterId),
	}
	return c.cache.Add(req.id, req)
}

func (c *ChunkDataPackRequestQueue) Pop() (flow.Identifier, flow.Identifier, bool) {
	head, ok := c.cache.Head()
	if !ok {
		// cache is empty, and there is no head yet to pop.
		return flow.Identifier{}, flow.Identifier{}, false
	}

	c.cache.Remove(head.ID())
	request := head.(chunkDataPackRequest)
	return request.requesterId, request.chunkId, true
}

func (c ChunkDataPackRequestQueue) Size() uint {
	return c.cache.Size()
}

type chunkDataPackRequest struct {
	chunkId     flow.Identifier
	requesterId flow.Identifier
	// caching identifier to avoid cpu overhead
	// per query.
	id flow.Identifier
}

var _ flow.Entity = &chunkDataPackRequest{}

func (c chunkDataPackRequest) ID() flow.Identifier {
	return c.id
}

func (c chunkDataPackRequest) Checksum() flow.Identifier {
	return c.id
}

func identifierOf(chunkId flow.Identifier, requesterId flow.Identifier) flow.Identifier {
	return flow.MakeID(chunkDataPackRequest{requesterId: requesterId, chunkId: chunkId})
}
