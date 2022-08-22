package topology

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// FullyConnectedTopology returns all nodes as the fanout.
type FullyConnectedTopology struct{}

var _ network.Topology = &FullyConnectedTopology{}

func NewFullyConnectedTopology() network.Topology {
	return &FullyConnectedTopology{}
}

func (f FullyConnectedTopology) Fanout(ids flow.IdentityList) flow.IdentityList {
	return ids
}
