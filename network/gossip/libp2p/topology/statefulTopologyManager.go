package topology

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p/channel"
)

type StatefulTopologyManager struct {
	fanout   FanoutFunc                  // used to keep track size of constructed topology
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
// Independent invocations of MakeTopology on different nodes collaboratively
// constructs a connected graph of nodes that enables them talking to each other.
func (stm *StatefulTopologyManager) MakeTopology(ids flow.IdentityList) (flow.IdentityList, error) {
	var myFanout flow.IdentityList

	// samples a connected component fanout from each topic and takes the
	// union of all fanouts.
	myChannelIDs := stm.subMngr.GetChannelIDs()
	if len(myChannelIDs) == 0 {
		// no subscribed channel id, hence skip topology creation
		return flow.IdentityList{}, nil
	}

	// finds all interacting roles with this node
	myInteractingRoles := flow.RoleList{}
	for _, myChannel := range myChannelIDs {
		roles, ok := engine.RolesByChannelID(myChannel)
		if !ok {
			return nil, fmt.Errorf("could not extract roles for channel %s", myChannel)
		}
		myInteractingRoles = myInteractingRoles.Union(roles)
	}
	//fmt.Println("roles:")

	for _, role := range myInteractingRoles {
		if role == flow.RoleCollection {
			continue
		}
		roleFanout, err := stm.topology.Subset(ids.Filter(filter.HasRole(role)), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to derive list of peer nodes to connect for role %s: %w", role, err)
		}
		myFanout = myFanout.Union(roleFanout)
		//fmt.Println(len(myFanout), role, "AN", len(myFanout.Filter(filter.HasRole(flow.RoleAccess))),
		//	"CON", len(myFanout.Filter(filter.HasRole(flow.RoleConsensus))),
		//	"COL", len(myFanout.Filter(filter.HasRole(flow.RoleCollection))),
		//	"EN", len(myFanout.Filter(filter.HasRole(flow.RoleExecution))),
		//	"VN", len(myFanout.Filter(filter.HasRole(flow.RoleVerification))))
	}

	//fmt.Println("channels:")

	for _, myChannel := range myChannelIDs {
		shouldHave := make([]*flow.Identity, len(myFanout))
		copy(shouldHave, myFanout)

		topicFanout, err := stm.topology.ChannelSubset(ids, nil, myChannel)
		if err != nil {
			return nil, fmt.Errorf("failed to derive list of peer nodes to connect for topic %s: %w", myChannel, err)
		}
		myFanout = myFanout.Union(topicFanout)
		// roles, _ := engine.RolesByChannelID(myChannel)
		//fmt.Println(len(myFanout), myChannel, roles, "AN", len(myFanout.Filter(filter.HasRole(flow.RoleAccess))),
		//	"CON", len(myFanout.Filter(filter.HasRole(flow.RoleConsensus))),
		//	"COL", len(myFanout.Filter(filter.HasRole(flow.RoleCollection))),
		//	"EN", len(myFanout.Filter(filter.HasRole(flow.RoleExecution))),
		//	"VN", len(myFanout.Filter(filter.HasRole(flow.RoleVerification))))
	}

	if len(myFanout) == 0 {
		return nil, fmt.Errorf("topology size reached zero")
	}
	return myFanout, nil
}

// MakeTopology receives identity list of entire network and constructs identity list of topology
// of this instance. A node directly communicates with its topology identity list on epidemic dissemination
// of the messages (i.e., publish and multicast).
// Independent invocations of MakeTopology on different nodes collaboratively
// constructs a connected graph of nodes that enables them talking to each other.
func (stm *StatefulTopologyManager) Fanout(size int) int {
	return stm.fanout(size)
}
