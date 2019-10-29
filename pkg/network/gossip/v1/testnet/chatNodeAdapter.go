package testnet

// chatNodeAdapter models an adaptor to run a gNode over the hasherNode, and to represent the hasherNode in the network

import (
	"errors"
	"fmt"
	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	"github.com/dapperlabs/flow-go/pkg/network/gossip/v1/protocols"
	"github.com/rs/zerolog"
)

// initializes a gnode with the given chatNode's functions as its registry
// and makes it serve a listener from the pool. If different functions using the function at the same time
// provide different startPorts to avoid conflicts
func (cn *chatNode) startNode(logger zerolog.Logger, fanoutSize int, totalNumNodes int, startPort int) (*gnode.Node, error) {
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

	config := gnode.NewNodeConfig(NewReceiverServerRegistry(cn), myPort, othersPort, fanoutSize, 10)
	node := gnode.NewNode(config)

	sp, err := protocols.NewGServer(node)
	if err != nil{
		return nil, errors.New("could not initialize new GServer")
	}

	node.SetProtocol(sp)

	go node.Serve(listener)

	return node, nil
}
