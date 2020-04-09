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
func ClusterFilterFor(sn Snapshot, id flow.Identifier) (flow.IdentityFilter, error) {

	clusters, err := sn.Clusters()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}
	cluster, ok := clusters.ByNodeID(id)
	if !ok {
		return nil, fmt.Errorf("could not get cluster for node")
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

func Clusters(nClusters uint, identities flow.IdentityList) *flow.ClusterList {

	// order the identities by node ID
	sort.Slice(identities.Filter(filter.HasRole(flow.RoleCollection)), func(i, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	// create the desired number of clusters and assign nodes
	clusters := flow.NewClusterList(nClusters)
	for i, identity := range identities {
		index := uint(i) % nClusters
		clusters.Add(index, identity)
	}

	return clusters
}
