package topology

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
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

// Subset samples and returns a connected graph fanout of the subscribers to the topic from the idList.
// A connected graph fanout means that the subset of ids returned by this method on different nodes collectively
// construct a connected graph component among all the subscribers to the topic.
// If topic is a cluster-related topic it samples only among the cluster nodes to which the collection node belongs to.
func (c CollectionTopology) Subset(idList flow.IdentityList, fanout uint, topic string) (flow.IdentityList, error) {
	if engine.IsClusterTopic(topic) {
		// extracts cluster peer ids to which the node belongs to
		clusterPeers, err := c.clusterPeers()
		if err != nil {
			return nil, fmt.Errorf("failed to find cluster peers for node %s", c.nodeID.String())
		}
		idList = clusterPeers
	}

	return c.TopicAwareTopology.Subset(idList, fanout, topic)
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
