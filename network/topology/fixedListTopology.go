package topology

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// FixedListTopology always returns the same node Identity list
type FixedListTopology struct {
	fixedNodeIdentities flow.IdentityList
}

func NewFixedListTopology(fixedNodeIdentities flow.IdentityList) FixedListTopology {
	return FixedListTopology{
		fixedNodeIdentities: fixedNodeIdentities,
	}
}

func (r FixedListTopology) GenerateFanout(ids flow.IdentityList, _ network.ChannelList) (flow.IdentityList, error) {
	return r.fixedNodeIdentities, nil
}

// EmptyListTopology always returns an empty list as the fanout
type EmptyListTopology struct {
}

func (r EmptyListTopology) GenerateFanout(_ flow.IdentityList, _ network.ChannelList) (flow.IdentityList, error) {
	return flow.IdentityList{}, nil
}
