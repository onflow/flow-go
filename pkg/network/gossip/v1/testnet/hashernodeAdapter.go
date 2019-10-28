package testnet

// hasherNodeAdapter models an adaptor to run a gNode over the hasherNode, and to represent the hasherNode in the network
import (
	"fmt"

	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	"github.com/rs/zerolog"
)

// startNode initializes a hasherNode. As part of the initialization, it creates a gNode, registers the receive method of the
// hasherNode on it, and returns the gNode instance. Any gossip with the message type of "Receive" for that instance of the gNode
// is routed to the corresponding instance of its hashNode
func (hn *hasherNode) startNode(logger zerolog.Logger, fanoutSize int, totalNumNodes int, startPort int) (*gnode.Node, error) {
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

	config := gnode.NewNodeConfig(nil, myPort, othersPort, fanoutSize, 10)
	node := gnode.NewNode(config)

	err = node.RegisterFunc("Receive", hn.receive)
	if err != nil {
		return nil, err
	}

	go node.Serve(listener)

	return node, nil
}
