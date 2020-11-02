package topology

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type StatefulTopologyManager struct {
	fanout   uint     // used to keep track of topology identity list of this instance
	topology Topology // used to sample nodes
}

// NewStatefulTopologyManager generates and returns an instance of stateful topology manager.
func NewStatefulTopologyManager(me flow.Identifier,
	state protocol.ReadOnlyState,
	fanout uint) (*StatefulTopologyManager, error) {

	// creates topology instance for manager
	top, err := NewTopicBasedTopology(me, state)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate topology: %w", err)
	}

	return &StatefulTopologyManager{
		fanout:   fanout,
		topology: top,
	}, nil
}

// MakeTopology receives identity list of entire network and constructs identity list of topology
// of this instance. A node directly communicates with its topology identity list on epidemic dissemination
// of the messages (i.e., publish and multicast).
// Independent invocations of MakeTopology on different nodes collaboratively
// constructs a connected graph of nodes that enables them talking to each other.
func (stm StatefulTopologyManager) MakeTopology(ids flow.IdentityList) (flow.IdentityList, error) {
	return flow.IdentityList{}, nil
}

// MakeTopology receives identity list of entire network and constructs identity list of topology
// of this instance. A node directly communicates with its topology identity list on epidemic dissemination
// of the messages (i.e., publish and multicast).
// Independent invocations of MakeTopology on different nodes collaboratively
// constructs a connected graph of nodes that enables them talking to each other.
func (stm StatefulTopologyManager) Fanout() uint {
	return stm.fanout
}
