package topology

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// TopicBasedTopology is a deterministic topology mapping that creates a connected graph component among the nodes
// involved in each topic.
type TopicBasedTopology struct {
	me           flow.Identifier        // used to keep identifier of the node
	state        protocol.ReadOnlyState // used to keep a read only protocol state
	notMeFilter  flow.IdentityFilter    // used to filter out the node itself
	graphSampler ConnectedGraphSampler  // used to create connected graph sampler
}

// NewTopicBasedTopology returns an instance of the TopicBasedTopology.
func NewTopicBasedTopology(nodeID flow.Identifier, state protocol.ReadOnlyState,
	graphSampler ConnectedGraphSampler) (*TopicBasedTopology, error) {
	t := &TopicBasedTopology{
		me:           nodeID,
		state:        state,
		graphSampler: graphSampler,
		notMeFilter:  filter.Not(filter.HasNodeID(nodeID)),
	}

	return t, nil
}

// Subset samples and returns a connected graph of the subscribers to the topic from the ids.
// A connected graph fanout means that the subset of ids returned by this method on different nodes collectively
// construct a connected graph component among all the subscribers to the topic.
func (t *TopicBasedTopology) Subset(ids flow.IdentityList, shouldHave flow.IdentityList, topic string) (flow.IdentityList, error) {
	var subscribers flow.IdentityList
	var involvedRoles flow.RoleList

	if _, ok := engine.IsClusterChannelID(topic); ok {
		// extracts cluster peer ids to which the node belongs to.
		clusterPeers, err := t.clusterPeers()
		if err != nil {
			return nil, fmt.Errorf("failed to find cluster peers for node %s: %w", t.me.String(), err)
		}

		subscribers = clusterPeers

		involvedRoles = flow.RoleList{flow.RoleCollection}
		shouldHave = shouldHave.Filter(filter.HasRole(flow.RoleCollection))
	} else {
		// not a cluster-based topic.
		//
		// extracts flow roles subscribed to topic.
		roles, ok := engine.RolesByChannelID(topic)
		if !ok {
			return nil, fmt.Errorf("unknown topic with no subscribed roles: %s", topic)
		}

		// extract ids of subscribers to the topic
		subscribers = ids.Filter(filter.HasRole(roles...))
		involvedRoles = roles
	}

	// excludes the node itself from its topology
	subscribers = subscribers.Filter(t.notMeFilter)

	if shouldHave != nil {
		// excludes irrelevant roles from should have set
		shouldHave = shouldHave.Filter(filter.HasRole(involvedRoles...))

		// excludes the node itself from its topology
		shouldHave = shouldHave.Filter(t.notMeFilter)
	}

	// samples subscribers of a connected graph
	subscriberSample := t.graphSampler.SampleConnectedGraph(subscribers, shouldHave)

	return subscriberSample, nil
}

// clusterPeers returns the list of other nodes within the same cluster as this node.
func (t TopicBasedTopology) clusterPeers() (flow.IdentityList, error) {
	currentEpoch := t.state.Final().Epochs().Current()
	clusterList, err := currentEpoch.Clustering()
	if err != nil {
		return nil, fmt.Errorf("failed to extract cluster list %w", err)
	}

	myCluster, _, found := clusterList.ByNodeID(t.me)
	if !found {
		return nil, fmt.Errorf("failed to find the cluster for node ID %s", t.me.String())
	}

	return myCluster, nil
}
