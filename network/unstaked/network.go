package unstaked

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

type UnstakedNetwork struct {
	module.Network
	stakedNodeID flow.Identifier
}

// NewUnstakedNetwork creates a new unstaked network. All messages sent on this network are
// sent only to the staked node identified by the given node ID.
func NewUnstakedNetwork(net module.Network, stakedNodeID flow.Identifier) *UnstakedNetwork {
	return &UnstakedNetwork{
		net,
		stakedNodeID,
	}
}

// Register registers an engine with the unstaked network.
func (n *UnstakedNetwork) Register(channel network.Channel, engine network.Engine) (network.Conduit, error) {
	con, err := n.Network.Register(channel, engine)

	if err != nil {
		return nil, err
	}

	unstakedCon := UnstakedConduit{
		con,
		n.stakedNodeID,
	}

	return &unstakedCon, nil
}
