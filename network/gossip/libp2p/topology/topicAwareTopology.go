package topology

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// TopicAwareTopology is a deterministic topology mapping that creates a connected graph component among the nodes
// involved in each topic.
type TopicAwareTopology struct {
	seed  int64                  // used for sampling connected graph
	me    flow.Identifier        // used to keep identifier of the node
	state protocol.ReadOnlyState // used to keep a read only protocol state
}

// NewTopicAwareTopology returns an instance of the TopicAwareTopology.
func NewTopicAwareTopology(nodeID flow.Identifier, state protocol.ReadOnlyState) (*TopicAwareTopology, error) {
	seed, err := seedFromID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to seed topology: %w", err)
	}
	t := &TopicAwareTopology{
		seed:  seed,
		me:    nodeID,
		state: state,
	}

	return t, nil
}

// Subset samples and returns a connected graph of the subscribers to the topic from the ids.
// A connected graph fanout means that the subset of ids returned by this method on different nodes collectively
// construct a connected graph component among all the subscribers to the topic.
func (t *TopicAwareTopology) Subset(ids flow.IdentityList, fanout uint, topic string) (flow.IdentityList, error) {
	var subscribers flow.IdentityList
	if engine.IsClusterTopic(topic) {
		// extracts cluster peer ids to which the node belongs to.
		clusterPeers, err := t.clusterPeers()
		if err != nil {
			return nil, fmt.Errorf("failed to find cluster peers for node %s", t.me.String())
		}

		subscribers = clusterPeers
	} else {
		// not a cluster-based topic.
		//
		// extracts flow roles subscribed to topic.
		roles, ok := engine.GetRolesByTopic(topic)
		if !ok {
			return nil, fmt.Errorf("unknown topic with no subscribed roles: %s", topic)
		}

		// extract ids of subscribers to the topic
		subscribers = ids.Filter(filter.HasRole(roles...))

		// excluding the node itself from its topology
		subscribers = subscribers.Filter(filter.Not(filter.HasNodeID(t.me)))
	}

	// samples subscribers of a connected graph
	subscriberSample, _ := connectedGraphSample(subscribers, t.seed)

	return subscriberSample, nil
}

// clusterPeers returns the list of other nodes within the same cluster as this node.
func (t TopicAwareTopology) clusterPeers() (flow.IdentityList, error) {
	currentEpoch := t.state.Final().Epochs().Current()
	clusterList, err := currentEpoch.Clustering()
	if err != nil {
		return nil, err
	}

	myCluster, _, found := clusterList.ByNodeID(t.me)
	if !found {
		return nil, fmt.Errorf("failed to find the cluster for node ID %s", t.me.String())
	}

	return myCluster, nil
}
