package proxy

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
)

type ProxyNetwork struct {
	targetNodeID flow.Identifier
	net          p2p.ReadyDoneAwareNetwork
}

// NewProxyNetwork creates a new proxy network. All messages sent on this network are
// sent only to the node identified by the given target ID.
func NewProxyNetwork(net p2p.ReadyDoneAwareNetwork, targetNodeID flow.Identifier) *ProxyNetwork {
	return &ProxyNetwork{
		net:          net,
		targetNodeID: targetNodeID,
	}
}

// Register registers an engine with the proxy network.
func (n *ProxyNetwork) Register(channel network.Channel, engine network.Engine) (network.Conduit, error) {
	con, err := n.net.Register(channel, engine)

	if err != nil {
		return nil, err
	}

	proxyCon := ProxyConduit{
		con,
		n.targetNodeID,
	}

	return &proxyCon, nil
}

func (n *ProxyNetwork) Ready() <-chan struct{} {
	return n.net.Ready()
}

func (n *ProxyNetwork) Done() <-chan struct{} {
	return n.net.Done()
}
