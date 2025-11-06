package ingestion2

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// Tracks missing collections per height and invokes job callbacks when complete.
type MissingCollectionQueue interface {
	EnqueueMissingCollections(blockHeight uint64, ids []flow.Identifier, callback func()) error
	OnIndexedForBlock(blockHeight uint64) // mark done (post‑indexing)

	// On receipt of a collection, MCQ updates internal state and, if a block
	// just became complete, returns: (collections, height, true).
	// Otherwise, returns (nil, 0, false).
	OnReceivedCollection(collection *flow.Collection) ([]*flow.Collection, uint64, bool)
}

type CollectionRequester interface {
	RequestCollections(ids []flow.Identifier) error
}

// Implements the job lifecycle for a single block height.
type JobProcessor interface {
	ProcessJob(ctx irrecoverable.SignalerContext, job module.Job, done func()) error
	OnReceivedCollectionsForBlock(blockHeight uint64, cols []*flow.Collection) error // called by EDI or requester
}
