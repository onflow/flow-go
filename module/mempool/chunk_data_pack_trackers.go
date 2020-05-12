package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// ChunkDataPackTrackers represents a concurrency-safe memory pool of chunk data pack trackers
type ChunkDataPackTrackers interface {

	// Has checks if the given chunkID has a tracker in mempool.
	Has(chunkID flow.Identifier) bool

	// Add will add the given chunk datapack tracker to the memory pool. It will
	// return  false if it was already in the mempool.
	Add(cdpt *tracker.ChunkDataPackTracker) bool

	// Rem removes tracker with the given chunk ID.
	Rem(chunkID flow.Identifier) bool

	// Inc atomically increases the counter of tracker by one and returns the updated tracker
	Inc(chunkID flow.Identifier) (*tracker.ChunkDataPackTracker, error)

	// ByChunkID returns the chunk data pack tracker for the given chunk ID.
	ByChunkID(chunkID flow.Identifier) (*tracker.ChunkDataPackTracker, bool)

	// All will return a list of chunk data pack trackers in mempool.
	All() []*tracker.ChunkDataPackTracker
}
