// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// HeadersGetter is read-only storage access for block headers and related indexes.
type HeadersGetter interface {
	// ByBlockID returns the header with the given ID. It is available for finalized and ambiguous blocks.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no block is known with the given ID
	ByBlockID(blockID flow.Identifier) (*flow.Header, error)

	// ByHeight returns the block with the given number. It is only available for finalized blocks.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no finalized block is known at given height
	ByHeight(height uint64) (*flow.Header, error)

	// BlockIDByHeight the block ID that is finalized at the given height. It is an optimized version
	// of `ByHeight` that skips retrieving the block.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no block is known at given height
	BlockIDByHeight(height uint64) (flow.Identifier, error)

	// ByParentID finds all children for the given parent block. The returned headers might be un-finalized.
	// If there is more than one, at least one of them has to be un-finalized.
	// If there are no children, an empty slice is returned (no error).
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no block is known with the given ID `parentID`
	ByParentID(parentID flow.Identifier) ([]*flow.Header, error)
}

// HeadersByChunkID is read/write storage for the chunkID->header index.
// These methods are exclusively used by the Execution Node.
type HeadersByChunkID interface {
	// IndexByChunkID indexes block ID by chunk ID.
	IndexByChunkID(headerID, chunkID flow.Identifier) error

	// BatchIndexByChunkID indexes block ID by chunk ID in a given batch.
	BatchIndexByChunkID(headerID, chunkID flow.Identifier, batch BatchStorage) error

	// IDByChunkID finds the ID of the block corresponding to given chunk ID.
	IDByChunkID(chunkID flow.Identifier) (flow.Identifier, error)

	// BatchRemoveChunkBlockIndexByChunkID removes block to chunk index entry keyed by a blockID in a provided batch
	// No errors are expected during normal operation, even if no entries are matched.
	// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
	BatchRemoveChunkBlockIndexByChunkID(chunkID flow.Identifier, batch BatchStorage) error
}

// Headers represents persistent read/write storage for block headers.
type Headers interface {
	// Store will store a header.
	Store(header *flow.Header) error

	HeadersGetter
	HeadersByChunkID
}
