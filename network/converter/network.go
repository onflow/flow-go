package converter

import (
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

type Network struct {
	module.ReadyDoneAwareNetwork
	from network.Channel
	to   network.Channel
}

func NewNetwork(net module.ReadyDoneAwareNetwork, from network.Channel, to network.Channel) *Network {
	return &Network{net, from, to}
}

func (n *Network) convert(channel network.Channel) network.Channel {
	if channel == n.from {
		return n.to
	}
	return channel
}

func (n *Network) Register(channel network.Channel, engine network.Engine) (network.Conduit, error) {
	return n.ReadyDoneAwareNetwork.Register(n.convert(channel), engine)
}
