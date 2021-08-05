package hotstuff

import "github.com/onflow/flow-go/consensus/hotstuff/model"

// VoteCollectorFactory creates vote collector.
type VoteCollectorFactory interface {
	// Create creates a vote collector for a certain block
	Create(block *model.Block) (VoteCollector, error)
}
