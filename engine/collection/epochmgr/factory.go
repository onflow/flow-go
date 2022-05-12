package epochmgr

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
)

// EpochComponentsFactory is responsible for creating epoch-scoped components
// managed by the epoch manager engine for the given epoch.
type EpochComponentsFactory interface {

	// Create sets up and instantiates all dependencies for the epoch. It may
	// be used either for an ongoing epoch (for example, after a restart) or
	// for an epoch that will start soon. It is safe to call multiple times for
	// a given epoch counter.
	//
	// Must return ErrNotAuthorizedForEpoch if this node is not authorized in the epoch.
	Create(epoch protocol.Epoch) (
		state cluster.State,
		proposal network.Engine,
		sync network.Engine,
		hotstuff module.HotStuff,
		voteAggregator hotstuff.VoteAggregator,
		err error,
	)
}
