package topology

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// CollectionTopology builds on top of RandPermTopology and generates a deterministic random topology for collection node
// such that nodes withing the same collection cluster form a connected graph.
type CollectionTopology struct {
	RandPermTopology
	nodeID flow.Identifier
	state  protocol.ReadOnlyState
	seed   int64
}

func NewCollectionTopology(nodeID flow.Identifier, state protocol.ReadOnlyState) (CollectionTopology, error) {
	rpt, err := NewRandPermTopology(flow.RoleCollection, nodeID)
	if err != nil {
		return CollectionTopology{}, err
	}
	return CollectionTopology{
		RandPermTopology: rpt,
		nodeID:           nodeID,
		state:            state,
	}, nil
}

// Subset samples the idList and returns a list of nodes to connect with such that:
// a. this node is directly or indirectly connected to all other nodes in the same cluster
// b. to all other nodes
// c. to at least one node of each type and
///d. to all other nodes of the same type.
// The collection nodes within a collection cluster need to form a connected graph among themselves independent of any
// other nodes to ensure reliable dissemination of cluster specific topic messages. e.g ClusterBlockProposal
// Similarly, all nodes of network need to form a connected graph, to ensure reliable dissemination of messages for
// topics subscribed by all node types e.g. BlockProposals
// Each node should be connected to at least one node of each type to ensure nodes don't form an island of a specific
// role, specially since some node types are represented by a very small number of nodes (e.g. few access nodes compared
//to tens or hundreds of collection nodes)
// Finally, all nodes of the same type should form a connected graph for exchanging messages for role specific topics
// e.g. Transaction
func (c CollectionTopology) Subset(idList flow.IdentityList, fanout uint, topic string) (flow.IdentityList, error) {

	if !strings.EqualFold(topic, DummyTopic) {
		return nil, fmt.Errorf("could not support topics, expected: %s, got %s", DummyTopic, topic)
	}

	randPermSample, err := c.RandPermTopology.Subset(idList, fanout, DummyTopic)
	if err != nil {
		return nil, err
	}

	clusterPeers, err := c.clusterPeers()
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster peers for node %s", c.nodeID.String())
	}
	clusterSample, _ := connectedGraphSample(clusterPeers, c.seed)

	// include only those cluster peers which have not already been chosen by RandPermTopology
	uniqueClusterSample := clusterSample.Filter(filter.Not(filter.In(randPermSample)))

	// add those to the earlier sample from randPerm
	randPermSample = append(randPermSample, uniqueClusterSample...)

	// return the aggregated set
	return randPermSample, nil
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
