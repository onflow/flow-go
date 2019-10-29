package testnet

// hasherNodeAdapter models an adaptor to run a gNode over the hasherNode, and to represent the hasherNode in the network
import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/network/gossip/protocols"
)

// startNode initializes a hasherNode. As part of the initialization, it creates a gNode, registers the receive method of the
// hasherNode on it, and returns the gNode instance. Any gossip with the message type of "Receive" for that instance of the gNode
// is routed to the corresponding instance of its hashNode
func (hn *hasherNode) startNode(logger zerolog.Logger, fanoutSize int, totalNumNodes int, startPort int) (*gossip.Node, error) {
	// the addresses of the nodes that we will use
	portPool := make([]string, totalNumNodes)

	for i := 0; i < totalNumNodes; i++ {
		// enumerating ports starting from startPort
		portPool[i] = fmt.Sprintf("127.0.0.1:%v", startPort+i)
	}

	listener, myPort, othersPort, err := pickPort(portPool)

	if err != nil {
		return nil, err
	}

	config := gossip.NewNodeConfig(nil, myPort, othersPort, fanoutSize, 10)
	node := gossip.NewNode(config)

	err = node.RegisterFunc("Receive", hn.receive)
	if err != nil {
		return nil, err
	}

	sp, err := protocols.NewGServer(node)
	if err != nil {
		return nil, errors.New("could not initialize new GServer")
	}

	node.SetProtocol(sp)

	go node.Serve(listener)

	return node, nil
}
