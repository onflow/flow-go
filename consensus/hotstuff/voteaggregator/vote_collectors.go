//nolint
package voteaggregator

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

type voteCollectors struct {
	lock       sync.RWMutex
	collectors map[uint64]hotstuff.VoteCollector // view -> VoteCollector
}

var _ hotstuff.VoteCollectors = &voteCollectors{}

// GetOrCreateCollector performs lazy initialization of collectors based on their view
func (v *voteCollectors) GetOrCreateCollector(view uint64) (hotstuff.VoteCollector, bool, error) {
	panic("implement me")
}

// PruneUpToView prunes all collectors which target same view
func (v *voteCollectors) PruneUpToView(view uint64) error {
	panic("implement me")
	return nil
}

func (v *voteCollectors) ProcessBlock(block *model.Proposal) error {
	panic("implement me")
}
