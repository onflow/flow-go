package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkHeaders represents persistent storage for chunk headers.
type ChunkHeaders interface {

	// Store inserts the chunk header, keyed by chunk ID.
	Store(c *flow.ChunkHeader) error

	// Remove removes the chunk header for the given chunk ID, if it exists.
	Remove(chunkID flow.Identifier) error

	// ByID returns the chunk header for the given chunk ID.
	ByID(chunkID flow.Identifier) (*flow.ChunkHeader, error)
}
