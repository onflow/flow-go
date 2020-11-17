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
	graphSampler ConnectedGraphSampler  // used to create connected graph sampler
}

// NewTopicBasedTopology returns an instance of the TopicBasedTopology.
func NewTopicBasedTopology(nodeID flow.Identifier, state protocol.ReadOnlyState,
	graphSampler ConnectedGraphSampler) (*TopicBasedTopology, error) {
	t := &TopicBasedTopology{
		me:           nodeID,
		state:        state,
		graphSampler: graphSampler,
	}

	return t, nil
}

// SubsetChannel returns a random subset of the identity list that is passed. `shouldHave` represents set of
// identities that should be included in the returned subset.
// Returned identities should all subscribed to the specified `channel`.
// Note: this method should not include identity of its executor.
func (t *TopicBasedTopology) SubsetChannel(ids flow.IdentityList, shouldHave flow.IdentityList,
	channel string) (flow.IdentityList, error) {
	if _, ok := engine.IsClusterChannelID(channel); ok {
		return t.clusterChannelHandler(ids, shouldHave)
	} else {
		return t.nonClusterChannelHandler(ids, shouldHave, channel)
	}
}

// SubsetRole returns a random subset of the identity list that is passed. `shouldHave` represents set of
// identities that should be included in the returned subset.
// Returned identities should all be of one of the specified `roles`.
// Note: this method should not include identity of its executor.
func (t TopicBasedTopology) SubsetRole(ids flow.IdentityList, shouldHave flow.IdentityList, roles flow.RoleList) (flow.IdentityList, error) {
	if shouldHave != nil {
		// excludes irrelevant roles from should have set
		shouldHave = shouldHave.Filter(filter.HasRole(roles...))

		// excludes the node itself from should have set
		shouldHave = shouldHave.Filter(filter.Not(filter.HasNodeID(t.me)))
	}

	// excludes the node itself from ids
	ids = ids.Filter(filter.Not(filter.HasNodeID(t.me)))

	sample, err := t.graphSampler.SampleConnectedGraph(ids, shouldHave)
	if err != nil {
		return nil, fmt.Errorf("could not sample a connected graph: %w", err)
	}

	return sample, nil
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

// clusterChannelHandler returns a connected graph fanout of peers in the same cluster as executor of this instance.
func (t TopicBasedTopology) clusterChannelHandler(ids, shouldHave flow.IdentityList) (flow.IdentityList, error) {
	// extracts cluster peer ids to which the node belongs to.
	clusterPeers, err := t.clusterPeers()
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster peers for node %s: %w", t.me.String(), err)
	}

	// samples a connected graph topology from the cluster peers
	return t.SubsetRole(clusterPeers, shouldHave.Filter(filter.HasNodeID(clusterPeers.NodeIDs()...)), flow.RoleList{flow.RoleCollection})
}

// clusterChannelHandler returns a connected graph fanout of peers from `ids` that subscribed to `channel`.
// The returned sample contains `shouldHave` ones that also subscribed to `channel`.
func (t TopicBasedTopology) nonClusterChannelHandler(ids, shouldHave flow.IdentityList, channel string) (flow.IdentityList, error) {
	if _, ok := engine.IsClusterChannelID(channel); ok {
		return nil, fmt.Errorf("could not handle cluster channel: %s", channel)
	}

	// extracts flow roles subscribed to topic.
	roles, ok := engine.RolesByChannelID(channel)
	if !ok {
		return nil, fmt.Errorf("unknown topic with no subscribed roles: %s", channel)
	}

	// samples a connected graph topology
	return t.SubsetRole(ids.Filter(filter.HasRole(roles...)), shouldHave, roles)
}
