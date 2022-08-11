package topology

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
)

const FixedList = Name("fixed-list")

func FixedListTopologyFactory() FactoryFunction {
	return func(nodeId flow.Identifier, _ zerolog.Logger, _ protocol.State, _ float64) (network.Topology, error) {
		return NewFixedListTopology(nodeId), nil
	}
}

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
