package collection_sync

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// Tracks missing collections per height and invokes job callbacks when complete.
type MissingCollectionQueue interface {
	EnqueueMissingCollections(blockHeight uint64, ids []flow.Identifier, callback func()) error
	OnIndexedForBlock(blockHeight uint64) bool // mark done (post‑indexing), returns true if height existed

	// On receipt of a collection, MCQ updates internal state and, if a block
	// just became complete, returns: (collections, height, true).
	// Otherwise, returns (nil, 0, false).
	OnReceivedCollection(collection *flow.Collection) ([]*flow.Collection, uint64, bool)

	// IsHeightQueued returns true if the given height is still being tracked (has not been indexed yet).
	IsHeightQueued(height uint64) bool
}

// Requests collections by their IDs.
type CollectionRequester interface {
	RequestCollections(ids []flow.Identifier) error
}

// BlockCollectionIndexer stores and indexes collections for a given block height.
type BlockCollectionIndexer interface {
	// IndexCollectionsForBlock stores and indexes collections for a given block height.
	IndexCollectionsForBlock(blockHeight uint64, cols []*flow.Collection) error
}

// Implements the job lifecycle for a single block height.
type JobProcessor interface {
	ProcessJobConcurrently(ctx irrecoverable.SignalerContext, job module.Job, done func()) error
}

// Fetcher is a component that consumes finalized block jobs and processes them
// to index collections. It uses a job consumer with windowed throttling to prevent node overload.
type Fetcher interface {
	component.Component
	OnFinalizedBlock()
	LastProcessedIndex() uint64
	Head() (uint64, error)
	Size() uint
}

// ExecutionDataProvider provides the latest height for which execution data indexer has collections.
// This can be nil if execution data indexing is disabled.
type ExecutionDataProvider interface {
	HighestIndexedHeight() uint64
	GetExecutionDataByHeight(ctx context.Context, height uint64) ([]*flow.Collection, error)
}

type ExecutionDataProcessor interface {
	OnNewExectuionData()
}
