package testnet

// chatNodeAdapter models an adaptor to run a gNode over the hasherNode, and to represent the hasherNode in the network

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/network/gossip/protocols"
)

// initializes a gnode with the given chatNode's functions as its registry
// and makes it serve a listener from the pool. If different functions using the function at the same time
// provide different startPorts to avoid conflicts
func (cn *chatNode) startNode(logger zerolog.Logger, fanoutSize int, totalNumNodes int, startPort int) (*gossip.Node, error) {
	// the addresses of the nodes that we will use
	portPool := make([]string, totalNumNodes)

	for i := 0; i < totalNumNodes; i++ {
		// enumarating ports starting from startPort
		portPool[i] = fmt.Sprintf("127.0.0.1:%v", startPort+i)
	}

	listener, myPort, othersPort, err := pickPort(portPool)

	if err != nil {
		return nil, err
	}

	config := gossip.NewNodeConfig(NewReceiverServerRegistry(cn), myPort, othersPort, fanoutSize, 10)
	node := gossip.NewNode(config)

	sp, err := protocols.NewGServer(node)
	if err != nil {
		return nil, errors.New("could not initialize new GServer")
	}

	node.SetProtocol(sp)

	go node.Serve(listener)

	return node, nil
}
