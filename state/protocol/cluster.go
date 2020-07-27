package protocol

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
)

// ClusterFilterFor returns a filter to retrieve all nodes within the cluster
// that the node with the given ID belongs to.
func ClusterFilterFor(sn Snapshot, nodeID flow.Identifier) (flow.IdentityFilter, uint, error) {

	clusters, err := sn.Clusters()
	if err != nil {
		return nil, 0, fmt.Errorf("could not get clusters: %w", err)
	}
	cluster, index, ok := clusters.ByNodeID(nodeID)
	if !ok {
		return nil, 0, fmt.Errorf("could not get cluster for node")
	}

	return filter.In(cluster), index, nil
}

// ClusterFor returns the cluster that the node with given ID belongs to.
func ClusterFor(sn Snapshot, nodeID flow.Identifier) (flow.IdentityList, uint, error) {

	clusterFilter, clusterIndex, err := ClusterFilterFor(sn, nodeID)
	if err != nil {
		return nil, 0, fmt.Errorf("could not get cluster filter: %w", err)
	}
	participants, err := sn.Identities(clusterFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("could not get nodes in cluster: %w", err)
	}

	return participants, clusterIndex, nil
}

// ClusterByIndex returns the cluster by the given index
func ClusterByIndex(sn Snapshot, index uint) (flow.IdentityList, error) {
	clusters, err := sn.Clusters()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}

	cluster, ok := clusters.ByIndex(index)
	if !ok {
		return nil, fmt.Errorf("could not find cluster by index: %v", index)
	}

	return cluster, nil
}

// ChainIDForCluster returns the canonical chain ID for a collection node cluster.
func ChainIDForCluster(cluster flow.IdentityList) flow.ChainID {
	return flow.ChainID(cluster.Fingerprint().String())
}

// ClusterAssignments returns the assignments for clusters based on the given
// number and the provided collector identities.
func ClusterAssignments(num uint, collectors flow.IdentityList) flow.AssignmentList {

	// double-check we only have collectors
	collectors = collectors.Filter(filter.HasRole(flow.RoleCollection))

	// order the identities by node ID
	sort.Slice(collectors, func(i, j int) bool {
		return order.ByNodeIDAsc(collectors[i], collectors[j])
	})

	// create the desired number of clusters and assign nodes
	var assignments flow.AssignmentList
	for i, collector := range collectors {
		index := uint(i) % num
		assignments[index] = append(assignments[index], collector.NodeID)
	}

	return assignments
}
