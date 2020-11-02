package topology

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/gossip/libp2p/channel"
	"github.com/onflow/flow-go/state/protocol"
)

type StatefulTopologyManager struct {
	sync.Mutex
	fanout   uint     // used to keep track of topology identity list of this instance
	topology Topology // used to sample nodes
	subMngr  channel.SubscriptionManager
}

// NewStatefulTopologyManager generates and returns an instance of stateful topology manager.
func NewStatefulTopologyManager(me flow.Identifier,
	state protocol.ReadOnlyState,
	subMngr channel.SubscriptionManager,
	fanout uint) (*StatefulTopologyManager, error) {

	// creates topology instance for manager
	top, err := NewTopicBasedTopology(me, state)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate topology: %w", err)
	}

	return &StatefulTopologyManager{
		fanout:   fanout,
		topology: top,
		subMngr:  subMngr,
	}, nil
}

// MakeTopology receives identity list of entire network and constructs identity list of topology
// of this instance. A node directly communicates with its topology identity list on epidemic dissemination
// of the messages (i.e., publish and multicast).
// Independent invocations of MakeTopology on different nodes collaboratively
// constructs a connected graph of nodes that enables them talking to each other.
func (stm *StatefulTopologyManager) MakeTopology(ids flow.IdentityList) (flow.IdentityList, error) {
	stm.Lock()
	defer stm.Unlock()

	myFanout := flow.IdentityList{}

	// extracts channel ids this node subscribed to
	myChannels := stm.subMngr.GetChannelIDs()

	// samples a connected component fanout from each topic and takes the
	// union of all fanouts.
	for _, myChannel := range myChannels {
		subset, err := stm.topology.Subset(ids, stm.fanout, myChannel)
		if err != nil {
			return nil, fmt.Errorf("failed to derive list of peer nodes to connect for topic %s: %w", channel, err)
		}
		myFanout = myFanout.Union(subset)
	}
	return myFanout, nil
}

// MakeTopology receives identity list of entire network and constructs identity list of topology
// of this instance. A node directly communicates with its topology identity list on epidemic dissemination
// of the messages (i.e., publish and multicast).
// Independent invocations of MakeTopology on different nodes collaboratively
// constructs a connected graph of nodes that enables them talking to each other.
func (stm *StatefulTopologyManager) Fanout() uint {
	return stm.fanout
}
