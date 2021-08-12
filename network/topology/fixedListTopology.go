package topology

import (
	"github.com/onflow/flow-go/model/flow"
	idFilter "github.com/onflow/flow-go/model/flow/filter/id"
	"github.com/onflow/flow-go/network"
)

// FixedListTopology always returns the same node ID as the fanout
type FixedListTopology struct {
	fixedNodeID flow.Identifier
}

func NewFixedListTopology(nodeID flow.Identifier) FixedListTopology {
	return FixedListTopology{
		fixedNodeID: nodeID,
	}
}

func (r FixedListTopology) GenerateFanout(ids flow.IdentifierList, _ network.ChannelList) (flow.IdentifierList, error) {
	return ids.Filter(idFilter.Is(r.fixedNodeID)), nil
}

// EmptyListTopology always returns an empty list as the fanout
type EmptyListTopology struct {
}

func (r EmptyListTopology) GenerateFanout(_ flow.IdentifierList, _ network.ChannelList) (flow.IdentifierList, error) {
	return flow.IdentifierList{}, nil
}
