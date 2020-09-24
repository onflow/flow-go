package topology

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// CollectionTopology builds on top of RandPermTopology and generates a deterministic random topology for collection node
// such that nodes withing the same collection cluster form a connected graph.
type CollectionTopology struct {
	RandPermTopology
	nodeID flow.Identifier
	state  protocol.State
	seed   int64
}

func NewCollectionTopology(nodeID flow.Identifier, state protocol.State) *CollectionTopology {
	return &CollectionTopology{
		RandPermTopology: NewRandPermTopology(flow.RoleCollection, nodeID),
		nodeID:           nodeID,
		state:            state,
	}
}

// Subset samples the idList and returns a list of nodes to connect with so that this node is directly or indirectly
// connected to all other nodes in the same cluster, to all other nodes, to atleast one node of each
// type and to all other nodes of the same type
func (c CollectionTopology) Subset(idList flow.IdentityList, fanout uint) (flow.IdentityList, error) {

	randPermSample, err := c.RandPermTopology.Subset(idList, fanout)
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

	clusterList.IndexOf()
	myCluster, _, found := clusterList.ByNodeID(c.nodeID)
	if !found {
		return nil, fmt.Errorf("failed to find the cluster for node ID %s", c.nodeID.String())
	}

	return myCluster, nil
}
