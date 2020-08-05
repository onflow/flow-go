package libp2p

import (
	"fmt"
	"net"

	"github.com/dapperlabs/flow-go/model/flow"
)

type nodeAddressMap struct {
	 idToAddressMap map[flow.Identifier]NodeAddress
}

func newNodeAddressMap(identityMap map[flow.Identifier]flow.Identity) (*nodeAddressMap, error) {
	idToAddressMap := make(map[flow.Identifier]NodeAddress)
	for id, identity := range identityMap {
		nodeAddress, err := nodeAddressFromID(identity)
		if err != nil {
			return nil, err
		}
		idToAddressMap[id] = nodeAddress
	}
	peerInfo := &nodeAddressMap{
		idToAddressMap: idToAddressMap,
	}
	return peerInfo, nil
}

// nodeAddressFromID returns the libp2p.NodeAddress for the given flow.identity
func nodeAddressFromID(flowIdentity flow.Identity) (NodeAddress, error) {

	// split the node address into ip and port
	ip, port, err := net.SplitHostPort(flowIdentity.Address)
	if err != nil {
		return NodeAddress{}, fmt.Errorf("could not parse address %s: %w", flowIdentity.Address, err)
	}

	// convert the Flow key to a LibP2P key
	lkey, err := PublicKey(flowIdentity.NetworkPubKey)
	if err != nil {
		return NodeAddress{}, fmt.Errorf("could not convert flow key to libp2p key: %w", err)
	}

	// create a new NodeAddress
	nodeAddress := NodeAddress{Name: flowIdentity.NodeID.String(), IP: ip, Port: port, PubKey: lkey}

	return nodeAddress, nil
}

func (n *nodeAddressMap) nodeAddress(id flow.Identifier) NodeAddress {
	return n.idToAddressMap[id]
}

