package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// ChunkStateTrackers represents a concurrency-safe memory pool of chunk states
type ChunkStatePackTrackers interface {

	// Add will add the given chunk state tracker.
	Add(cst *tracker.ChunkStateTracker) error

	// Has checks if the given chunkID has a tracker in mempool.
	Has(chunkID flow.Identifier) bool

	// Rem removes tracker with the given chunk ID.
	Rem(chunkID flow.Identifier) bool

	// ByChunkID returns the chunk state tracker for the given chunk ID.
	ByChunkID(chunkID flow.Identifier) (*tracker.ChunkStateTracker, error)

	// All will return a list of chunk state trackers in mempool.
	All() []*tracker.ChunkStateTracker
}
