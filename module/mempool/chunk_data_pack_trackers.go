package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// ChunkDataPackTrackers represents a concurrency-safe memory pool of chunk data pack trackers
type ChunkDataPackTrackers interface {

	// Add will add the given chunk data pack tracker.
	Add(cdpt *tracker.ChunkDataPackTracker) error

	// Has checks if the given chunkID has a tracker in mempool.
	Has(chunkID flow.Identifier) bool

	// Rem removes tracker with the given chunk ID.
	Rem(chunkID flow.Identifier) bool

	// ByChunkID returns the chunk data pack tracker for the given chunk ID.
	ByChunkID(chunkID flow.Identifier) (*tracker.ChunkDataPackTracker, error)

	// All will return a list of chunk data pack trackers in mempool.
	All() []*tracker.ChunkDataPackTracker
}
