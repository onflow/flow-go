package voteaggregator

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

type voteCollectors struct {
	lock             sync.RWMutex
	viewToBlockIDSet map[uint64]map[flow.Identifier]struct{}    // for pruning
	collectors       map[flow.Identifier]hotstuff.VoteCollector // blockID -> VoteCollector
}

// GetOrCreateCollector performs lazy initialization of collectors based on their view and blockID
func (v *voteCollectors) GetOrCreateCollector(view uint64, blockID flow.Identifier) (hotstuff.VoteCollector, error) {
	panic("implement me")
}

// PruneByView prunes all collectors which target same view
func (v *voteCollectors) PruneByView(view uint64) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	for blockID := range v.viewToBlockIDSet[view] {
		delete(v.collectors, blockID)
	}
	return nil
}

func (v *voteCollectors) ProcessBlock(block *model.Block) error {
	panic("implement me")
}
