package epochmgr

import (
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// EpochComponentsFactory TODO
type EpochComponentsFactory interface {
	// Setup TODO
	Create(epoch protocol.Epoch) (
		state cluster.State,
		proposal module.Engine,
		sync module.Engine,
		hotstuff module.HotStuff,
		err error,
	)
}
