package collection_sync

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// Tracks missing collections per height and invokes job callbacks when complete.
type MissingCollectionQueue interface {
	// EnqueueMissingCollections tracks the given missing collection IDs for the given block height.
	EnqueueMissingCollections(blockHeight uint64, ids []flow.Identifier, callback func()) error

	// OnIndexedForBlock returns the callback function for the given block height
	OnIndexedForBlock(blockHeight uint64) (func(), bool)

	// On receipt of a collection, MCQ updates internal state and, if a block
	// just became complete, returns: (collections, height, missingCollectionID, true).
	// Otherwise, returns (nil, height, missingCollectionID, false).
	// missingCollectionID is an arbitrary ID from the remaining missing collections, or ZeroID if none.
	OnReceivedCollection(collection *flow.Collection) ([]*flow.Collection, uint64, flow.Identifier, bool)

	// PruneUpToHeight removes all tracked heights up to and including the given height.
	PruneUpToHeight(height uint64) (callbacks []func())

	// IsHeightQueued returns true if the given height is still being tracked (has not been indexed yet).
	IsHeightQueued(height uint64) bool

	// Size returns the number of missing collections currently in the queue.
	Size() uint
}

// Requests collections by their guarantees.
type CollectionRequester interface {
	RequestCollectionsByGuarantees(guarantees []*flow.CollectionGuarantee) error
}

// BlockCollectionIndexer stores and indexes collections for a given block height.
type BlockCollectionIndexer interface {
	// IndexCollectionsForBlock stores and indexes collections for a given block height.
	// No error is exepcted during normal operation.
	IndexCollectionsForBlock(blockHeight uint64, cols []*flow.Collection) error

	// GetMissingCollections retrieves the block and returns collection guarantees that whose collections
	// are missing in storage.
	// Only garantees whose collections that are not already in storage are returned.
	GetMissingCollections(block *flow.Block) ([]*flow.CollectionGuarantee, error)
}

// BlockProcessor processes blocks to fetch and index their collections.
type BlockProcessor interface {
	// RequestCollectionsForBlock requests all missing collections for the given block.
	FetchCollections(ctx irrecoverable.SignalerContext, block *flow.Block, done func()) error
	// MissingCollectionQueueSize returns the number of missing collections currently in the queue.
	MissingCollectionQueueSize() uint
	// PruneUpToHeight removes all tracked heights up to and including the given height.
	PruneUpToHeight(height uint64)
}

// Fetcher is a component that consumes finalized block jobs and processes them
// to index collections. It uses a job consumer with windowed throttling to prevent node overload.
type Fetcher interface {
	component.Component
	ProgressReader
	OnFinalizedBlock()
	Size() uint
}

// ExecutionDataProvider provides the latest height for which execution data indexer has collections.
// This can be nil if execution data indexing is disabled.
type ExecutionDataProvider interface {
	HighestIndexedHeight() uint64
	GetExecutionDataByHeight(ctx context.Context, height uint64) ([]*flow.Collection, error)
}

// ExecutionDataProcessor processes execution data when new execution data is available.
type ExecutionDataProcessor interface {
	OnNewExectuionData()
}

// ProgressReader provides the current progress of collection fetching/indexing.
type ProgressReader interface {
	ProcessedHeight() uint64
}
