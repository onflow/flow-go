package topology

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// FullyConnectedTopology returns all nodes as a fanout.
type FullyConnectedTopology struct{}

func NewFullyConnectedTopology() network.Topology {
	return &FullyConnectedTopology{}
}

func (f *FullyConnectedTopology) GenerateFanout(ids flow.IdentityList, _ network.ChannelList) (flow.IdentityList, error) {
	return ids, nil
}
