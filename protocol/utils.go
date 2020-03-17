package protocol

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
)

// ClusterFilterFor returns a filter to retrieve all nodes within the cluster
// that the node with the given ID belongs to.
func ClusterFilterFor(sn Snapshot, id flow.Identifier) (flow.IdentityFilter, error) {

	clusters, err := sn.Clusters()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}
	cluster, err := clusters.ByNodeID(id)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster for node: %w", err)
	}

	return filter.In(cluster), nil
}

// ClusterFor returns the cluster that the node with given ID belongs to.
func ClusterFor(sn Snapshot, id flow.Identifier) (flow.IdentityList, error) {

	clusterFilter, err := ClusterFilterFor(sn, id)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster filter: %w", err)
	}
	participants, err := sn.Identities(clusterFilter)
	if err != nil {
		return nil, fmt.Errorf("could not get nodes in cluster: %w", err)
	}

	return participants, nil
}

// ChainIDForCluster returns the canonical chain ID for a collection node cluster.
func ChainIDForCluster(cluster flow.IdentityList) string {
	return cluster.Fingerprint().String()
}
