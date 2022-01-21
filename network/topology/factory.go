package topology

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
)

type Name string

type FactoryFunction func(flow.Identifier, zerolog.Logger, protocol.State, float64) (network.Topology, error)

func Factory(name Name) (FactoryFunction, error) {
	switch name {
	case FullyConnected:
		return FullyConnectedTopologyFactory(), nil
	case FixedList:
		return FixedListTopologyFactory(), nil
	case Randomized:
		return RandomizedTopologyFactory(), nil
	case TopicBased:
		return TopicBasedTopologyFactory(), nil
	default:
		return nil, fmt.Errorf("unknown topology name: %s", name)
	}
}
