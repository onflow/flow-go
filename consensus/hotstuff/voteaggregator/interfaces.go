package voteaggregator

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// LazyInitCollector is a helper structure that is used to return collector which is lazy initialized
type LazyInitCollector struct {
	Collector hotstuff.VoteCollector
	Created   bool // whether collector was created or retrieved from cache
}

type VoteCollectors interface {
	// GetOrCreateCollector performs lazy initialization of hotstuff.VoteCollector
	// collector is indexed by blockID and view
	GetOrCreateCollector(view uint64, blockID flow.Identifier) (LazyInitCollector, error)
	// PruneByView prunes already stored collectors by view.
	PruneByView(view uint64) error
	// ProcessBlock processes already validated block.
	// Calling this function will mark conflicting collectors as stale and change state of valid collectors
	ProcessBlock(block *model.Block) error
}
