//nolint
package voteaggregator

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

type blockKey string

type voteCollectors struct {
	lock sync.RWMutex

	// for pruning by view
	blockKeysByView map[uint64]map[blockKey]struct{}

	// map of string(blockID+view) -> VoteCollector state machine
	// collector are indexed by a block key, which is a combination of blockID and view.
	// This is to handle the case where an invalid vote might have invalid view.
	// If we have two different votes with the same blockID and different view,
	// we will create two different VoteCollectors for each. When we receive the block, and
	// learned one of the votes is valid, its VoteCollector will become VoteCollectorStatusVerifying,
	// whereas the VoteCollector for the invalid vote will become VoteCollectorStatusInvalid.
	collectors map[blockKey]hotstuff.VoteCollector
}

// GetOrCreateCollector performs lazy initialization of collectors based on their view and blockID
func (v *voteCollectors) GetOrCreateCollector(view uint64, blockID flow.Identifier) (hotstuff.VoteCollector, bool, error) {
	panic("implement me")
}

// PruneUpToView prunes all collectors which target same view
func (v *voteCollectors) PruneUpToView(view uint64) error {
	panic("implement me")
}

func (v *voteCollectors) ProcessBlock(block *model.Block) error {
	panic("implement me")
}

func getBlockKey(view uint64, blockID flow.Identifier) blockKey {
	key := fmt.Sprintf("%v-%v", blockID, view)
	return blockKey(key)
}
