package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// IngestedChunkIDs represents a concurrency-safe memory pool for ingested chunk IDs.
// By ingested chunk IDs we mean those that have a verifiable chunk for them forwarded from
// Ingest engine to the Verify engine of Verification node
type IngestedChunkIDs interface {
	// Has checks whether the mempool has the chunk ID
	Has(chunk *flow.Chunk) bool

	// Add will add the given chunk ID to the memory pool or it will error if
	// the chunk ID is already in the memory pool.
	Add(chunk *flow.Chunk) error

	// Rem will remove the given chunk ID from the memory pool; it will
	// return true if the chunk ID was known and removed.
	Rem(chunkID flow.Identifier) bool

	// All will retrieve all chunk IDs that are currently in the memory pool
	// as an IdentityList
	All() flow.IdentifierList
}
