package topology

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/gossip/libp2p/channel"
)

// StatefulTopologyManager implements Manager interface.
// The term stateful denotes the fact that it tries to optimize fanout by constructing topology state-by-state.
// Where the next state of topology utilizes nodes chosen in the current state as much as possible, hence to
// avoid unnecessary redundancies.
type StatefulTopologyManager struct {
	topology Topology                    // used to sample nodes
	subMngr  channel.SubscriptionManager // used to keep track topics the node subscribed to
}

// NewStatefulTopologyManager generates and returns an instance of stateful topology manager.
func NewStatefulTopologyManager(topology Topology, subMngr channel.SubscriptionManager) *StatefulTopologyManager {
	return &StatefulTopologyManager{
		topology: topology,
		subMngr:  subMngr,
	}
}

// MakeTopology receives identity list of entire network and constructs identity list of topology
// of this instance. A node directly communicates with its topology identity list on epidemic dissemination
// of the messages (i.e., publish and multicast).
// Independent invocations of MakeTopology on different nodes collaboratively must construct a cohesive
// connected graph of nodes that enables them talking to each other.
func (stm *StatefulTopologyManager) MakeTopology(ids flow.IdentityList) (flow.IdentityList, error) {
	myChannelIDs := stm.subMngr.GetChannelIDs()
	if len(myChannelIDs) == 0 {
		// no subscribed channel id, hence skip topology creation
		// we do not return an error at this state as invocation of MakeTopology may happen before
		// node subscribing to all its channels.
		return flow.IdentityList{}, nil
	}

	// finds all interacting roles with this node
	myInteractingRoles := flow.RoleList{}
	for _, myChannel := range myChannelIDs {
		roles, ok := engine.RolesByChannelID(myChannel)
		if !ok {
			return nil, fmt.Errorf("could not extract roles for channel: %s", myChannel)
		}
		myInteractingRoles = myInteractingRoles.Union(roles)
	}

	// builds a connected component per role this node interact with,
	var myFanout flow.IdentityList
	for _, role := range myInteractingRoles {
		if role == flow.RoleCollection {
			// we do not build connected component for collection nodes based on their role
			// rather we build it based on their cluster identity in the next step.
			continue
		}
		roleFanout, err := stm.topology.SubsetRole(ids, nil, flow.RoleList{role})
		if err != nil {
			return nil, fmt.Errorf("failed to derive list of peer nodes to connect for role %s: %w", role, err)
		}
		myFanout = myFanout.Union(roleFanout)
	}

	// stitches the role-based components that subscribed to the same channel id together.
	for _, myChannel := range myChannelIDs {
		shouldHave := make([]*flow.Identity, len(myFanout))
		copy(shouldHave, myFanout)

		topicFanout, err := stm.topology.SubsetChannel(ids, shouldHave, myChannel)
		if err != nil {
			return nil, fmt.Errorf("failed to derive list of peer nodes to connect for topic %s: %w", myChannel, err)
		}
		myFanout = myFanout.Union(topicFanout)
	}

	if len(myFanout) == 0 {
		return nil, fmt.Errorf("topology size reached zero")
	}
	return myFanout, nil
}
