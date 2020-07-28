package protocol

import (
	"sort"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
)

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
