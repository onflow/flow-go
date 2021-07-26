package topology

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
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

func (r FixedListTopology) GenerateFanout(ids flow.IdentityList, _ network.ChannelList) (flow.IdentityList, error) {
	return ids.Filter(filter.HasNodeID(r.fixedNodeID)), nil
}

// EmptyListTopology always returns an empty list as the fanout
type EmptyListTopology struct {
}

func (r EmptyListTopology) GenerateFanout(_ flow.IdentityList, _ network.ChannelList) (flow.IdentityList, error) {
	return flow.IdentityList{}, nil
}
