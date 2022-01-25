package topology

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
)

const FullyConnected = Name("fully-connected")

// FullyConnectedTopology returns all nodes as a fanout.
type FullyConnectedTopology struct{}

func FullyConnectedTopologyFactory() FactoryFunction {
	return func(_ flow.Identifier, _r zerolog.Logger, _ protocol.State, _ float64) (network.Topology, error) {
		return NewFullyConnectedTopology(), nil
	}
}

func NewFullyConnectedTopology() network.Topology {
	return &FullyConnectedTopology{}
}

func (f *FullyConnectedTopology) GenerateFanout(ids flow.IdentityList, _ network.ChannelList) (flow.IdentityList, error) {
	return ids, nil
}
