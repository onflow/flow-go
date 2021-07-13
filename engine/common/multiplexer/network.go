package multiplexer

import (
	"fmt"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

type Network struct {
	net      module.Network
	eng      Engine
	conduits map[network.Channel]network.Conduit // stores conduits for all registered channels
}

func NewNetwork(
	net module.Network,
	eng Engine,
) (*Network, error) {
	e := &Network{
		net:      net,
		eng:      eng,
		conduits: make(map[network.Channel]network.Conduit),
	}

	return e, nil
}

// Register will subscribe the given engine with the multiplexer on the given channel, and all registered
// engines will be notified with incoming messages on the channel.
// The returned Conduit can be used to send messages to engines on other nodes subscribed to the same channel
func (n *Network) Register(channel network.Channel, engine network.Engine) (network.Conduit, error) {
	err := n.eng.RegisterEngine(channel, engine)

	if err != nil {
		return nil, fmt.Errorf("failed to register engine with multiplexer: %w", err)
	}

	_, ok := n.conduits[channel]

	if !ok {
		conduit, err := n.net.Register(channel, &n.eng)

		if err != nil {
			n.eng.UnregisterEngine(channel, engine)
			return nil, fmt.Errorf("failed to register multiplexer engine on channel %s: %w", channel, err)
		}

		n.conduits[channel] = conduit
	}

	return n.conduits[channel], nil
}
