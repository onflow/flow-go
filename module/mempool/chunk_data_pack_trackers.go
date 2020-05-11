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

	// ByID retrieve the chonk datapack tracker with the given chunk ID from the
	// memory pool. It will return false if it was not found in the mempool.
	ByChunkID(chunkID flow.Identifier) (*tracker.ChunkDataPackTracker, bool)

	// All will return a list of chunk data pack trackers in mempool.
	All() []*tracker.ChunkDataPackTracker
}
