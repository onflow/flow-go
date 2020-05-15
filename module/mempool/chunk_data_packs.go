// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkDataPacks represents a concurrency-safe memory pool for chunk data packs.
type ChunkDataPacks interface {

	// Has checks whether the ChunkDataPack with the given chunkID is currently in
	// the memory pool.
	Has(chunkID flow.Identifier) bool

	// Add will add the given chunk datapack to the memory pool. It will return
	// false if it was already in the mempool.
	Add(cdp *flow.ChunkDataPack) bool

	// Rem will remove the given ChunkDataPack from the memory pool; it will
	// return true if the ChunkDataPack was known and removed.
	Rem(chunkID flow.Identifier) bool

	// ByID retrieve the chunk datapacke with the given chunk ID from the memory
	// pool. It will return false if it was not found in the mempool.
	ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all ChunkDataPacks that are currently in the memory pool
	// as a slice.
	All() []*flow.ChunkDataPack

	// Hash will return a hash of the contents of the memory pool.
	Hash() flow.Identifier
}
