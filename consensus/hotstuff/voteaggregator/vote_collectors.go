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

func (v *voteCollectors) GetOrCreateCollector(view uint64, blockID flow.Identifier) (hotstuff.VoteCollector, error) {
	panic("implement me")
}

func (v *voteCollectors) PruneByView(view uint64) error {
	panic("implement me")
}

func (v *voteCollectors) ProcessBlock(block *model.Block) error {
	panic("implement me")
}
