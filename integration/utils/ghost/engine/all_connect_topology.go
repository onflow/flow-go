package engine

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

var _ middleware.Topology = &AllConnectTopology{}

// AllConnectTopology returns all the nodes as the target ids to connect to
// It is allows the current node to be directly connected to all the nodes
// NOTE: To be used only for testing
type AllConnectTopology struct {
}

func NewAllConnectTopology() middleware.Topology {
	return &AllConnectTopology{}
}

func (a AllConnectTopology) Subset(idList flow.IdentityList, _ int, _ string) (map[flow.Identifier]flow.Identity, error) {

	// creates a map of all the ids
	topMap := make(map[flow.Identifier]flow.Identity, len(idList))
	for _, v := range idList {
		topMap[v.NodeID] = *v
	}

	return topMap, nil
}
