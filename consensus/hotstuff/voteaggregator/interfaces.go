package voteaggregator

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// voteCollectors is an interface which allows VoteAggregator to interact with collectors structured by
// view and blockID.
// Implementations of this interface are responsible for state transitions of `VoteCollector`s and pruning of
// stale and outdated collectors by view.
type VoteCollectors interface {
	// GetOrCreateCollector performs lazy initialization of hotstuff.VoteCollector
	// collector is indexed by blockID and view
	GetOrCreateCollector(view uint64, blockID flow.Identifier) (hotstuff.VoteCollector, error)
	// PruneByView prunes already stored collectors by view.
	PruneByView(view uint64) error
	// ProcessBlock performs validation of block signature and processes block with respected collector.
	// Calling this function will mark conflicting collectors as stale and change state of valid collectors
	ProcessBlock(block *model.Block) error
}
