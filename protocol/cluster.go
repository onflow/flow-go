package protocol

import (
	"sort"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
)

// Cluster sorts a list of collection nodes into a number of clusters.
// This creates clusters of size 1 (for MVP).
//
// This is guaranteed to be deterministic for the same input, regardless of
// sorting.
func Cluster(nodes flow.IdentityList) flow.ClusterList {
	sort.Slice(nodes, func(i, j int) bool {
		return identity.ByNodeIDAsc(nodes[i], nodes[j])
	})
	clusters := make(flow.ClusterList, len(nodes))

	for i, node := range nodes {
		clusters[i] = flow.IdentityList{node}
	}

	return clusters
}
