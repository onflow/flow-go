package channel_reassigner

import (
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

type ChannelReassignerNetwork struct {
	module.ReadyDoneAwareNetwork
	from network.Channel
	to   network.Channel
}

func NewChannelReassignerNetwork(net module.ReadyDoneAwareNetwork, from, to network.Channel) *ChannelReassignerNetwork {
	return &ChannelReassignerNetwork{net, from, to}
}

func (n *ChannelReassignerNetwork) Register(channel network.Channel, engine network.Engine) (network.Conduit, error) {
	reassignedChannel := channel
	if channel == n.from {
		reassignedChannel = n.to
	}

	return n.ReadyDoneAwareNetwork.Register(reassignedChannel, engine)
}
