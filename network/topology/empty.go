package topology

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// EmptyTopology always returns an empty fanout list.
type EmptyTopology struct{}

var _ network.Topology = &EmptyTopology{}

func NewEmptyTopology() network.Topology {
	return &EmptyTopology{}
}

func (e EmptyTopology) Fanout(_ flow.IdentityList) flow.IdentityList {
	return flow.IdentityList{}
}
