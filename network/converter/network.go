package converter

import (
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

type Network struct {
	network.Network
	from channels.Channel
	to   channels.Channel
}

func NewNetwork(net network.Network, from channels.Channel, to channels.Channel) *Network {
	return &Network{net, from, to}
}

func (n *Network) convert(channel channels.Channel) channels.Channel {
	if channel == n.from {
		return n.to
	}
	return channel
}

func (n *Network) Register(channel channels.Channel, engine network.MessageProcessor) (network.Conduit, error) {
	return n.Network.Register(n.convert(channel), engine)
}
