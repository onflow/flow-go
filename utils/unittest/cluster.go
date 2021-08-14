package unittest

import (
	"fmt"
	"sort"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
)

// TransactionForCluster generates a transaction that will be assigned to the
// target cluster ID.
func TransactionForCluster(clusters flow.ClusterList, target flow.IdentityList) flow.TransactionBody {
	tx := TransactionBodyFixture()
	return AlterTransactionForCluster(tx, clusters, target, func(*flow.TransactionBody) {})
}

// AlterTransactionForCluster modifies a transaction nonce until it is assigned
// to the target cluster.
//
// The `after` function is run after each modification to allow for any content
// dependent changes to the transaction (eg. signing it).
func AlterTransactionForCluster(tx flow.TransactionBody, clusters flow.ClusterList, target flow.IdentityList, after func(tx *flow.TransactionBody)) flow.TransactionBody {

	// Bound to avoid infinite loop in case the routing algorithm is broken
	for i := 0; i < 10000; i++ {
		tx.Script = append(tx.Script, '/', '/')

		if after != nil {
			after(&tx)
		}
		routed, ok := clusters.ByTxID(tx.ID())
		if !ok {
			panic(fmt.Sprintf("unable to find cluster by txID: %x", tx.ID()))
		}

		if routed.Fingerprint() == target.Fingerprint() {
			return tx
		}
	}

	panic(fmt.Sprintf("unable to find transaction for target (%x) with %d clusters", target, len(clusters)))
}

// ClusterAssignment creates an assignment list with n clusters and with nodes
// evenly distributed among clusters.
func ClusterAssignment(n uint, nodes flow.IdentityList) flow.AssignmentList {

	collectors := nodes.Filter(filter.HasRole(flow.RoleCollection))

	// order, so the same list results in the same
	sort.Slice(collectors, func(i, j int) bool {
		return order.ByNodeIDAsc(collectors[i], collectors[j])
	})

	assignments := make(flow.AssignmentList, n)
	for i, collector := range collectors {
		index := uint(i) % n
		assignments[index] = append(assignments[index], collector.NodeID)
	}

	return assignments
}

func ClusterList(n uint, nodes flow.IdentityList) flow.ClusterList {
	assignments := ClusterAssignment(n, nodes)
	clusters, err := flow.NewClusterList(assignments, nodes.Filter(filter.HasRole(flow.RoleCollection)))
	if err != nil {
		panic(err)
	}

	return clusters
}
