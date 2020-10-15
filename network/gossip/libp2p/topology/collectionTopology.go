package topology

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// CollectionTopology builds on top of RandPermTopology and generates a deterministic random topology for collection node
// such that nodes withing the same collection cluster form a connected graph.
type CollectionTopology struct {
	*TopicAwareTopology
	nodeID flow.Identifier
	state  protocol.ReadOnlyState
	seed   int64
}

func NewCollectionTopology(nodeID flow.Identifier, state protocol.ReadOnlyState) (CollectionTopology, error) {
	tat, err := NewTopicAwareTopology(nodeID)
	if err != nil {
		return CollectionTopology{}, err
	}
	return CollectionTopology{
		TopicAwareTopology: tat,
		nodeID:             nodeID,
		state:              state,
	}, nil
}

// Subset samples the idList and returns a list of nodes to connect with such that:
// a. this node is directly or indirectly connected to all other nodes in the same cluster
// b. to all other nodes that it shares a subscription topic
// The collection nodes within a collection cluster need to form a connected graph among themselves independent of any
// other nodes to ensure reliable dissemination of cluster specific topic messages. e.g ClusterBlockProposal
// Similarly, they should maintain a connected graph component with each node they share a topic,
// to ensure reliable dissemination of messages.
func (c CollectionTopology) Subset(idList flow.IdentityList, fanout uint, _ string) (flow.IdentityList, error) {
	var subset flow.IdentityList

	// extracts cluster peer ids to which the node belongs to
	clusterPeers, err := c.clusterPeers()
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster peers for node %s", c.nodeID.String())
	}

	// samples a connected graph fanout from the cluster peers set
	clusterSample, _ := connectedGraphSample(clusterPeers, c.seed)
	subset = append(subset, clusterSample...)

	// extracts topics to which this collection node belongs to
	topics := engine.GetTopicsByRole(flow.RoleCollection)

	for _, topic := range topics {
		topicSample, err := c.TopicAwareTopology.Subset(idList, 0, topic)
		if err != nil {
			return nil, fmt.Errorf("could not sample topology for topic %s", topic)
		}

		// extracts unique samples not existing in subset
		uniqueSample := topicSample.Filter(filter.Not(filter.In(subset)))

		// merges unique samples with the subset
		subset = append(subset, uniqueSample...)
	}

	return subset, nil
}

// clusterPeers returns the list of other nodes within the same cluster as this node
func (c CollectionTopology) clusterPeers() (flow.IdentityList, error) {
	currentEpoch := c.state.Final().Epochs().Current()
	clusterList, err := currentEpoch.Clustering()
	if err != nil {
		return nil, err
	}

	myCluster, _, found := clusterList.ByNodeID(c.nodeID)
	if !found {
		return nil, fmt.Errorf("failed to find the cluster for node ID %s", c.nodeID.String())
	}

	return myCluster, nil
}
