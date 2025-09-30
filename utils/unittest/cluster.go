package unittest

import (
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
)

// TransactionForCluster generates a transaction that will be assigned to the
// target cluster ID.
func TransactionForCluster(clusters flow.ClusterList, target flow.IdentitySkeletonList) flow.TransactionBody {
	tx := TransactionBodyFixture()
	return AlterTransactionForCluster(tx, clusters, target, func(*flow.TransactionBody) {})
}

// AlterTransactionForCluster modifies a transaction nonce until it is assigned
// to the target cluster.
//
// The `after` function is run after each modification to allow for any content
// dependent changes to the transaction (eg. signing it).
func AlterTransactionForCluster(tx flow.TransactionBody, clusters flow.ClusterList, target flow.IdentitySkeletonList, after func(tx *flow.TransactionBody)) flow.TransactionBody {

	// Bound to avoid infinite loop in case the routing algorithm is broken
	for i := 0; i < 10000; i++ {
		tx.Script = append(tx.Script, '/', '/')

		if after != nil {
			after(&tx)
		}
		routed, ok := clusters.ByTxID(tx.Hash())
		if !ok {
			panic(fmt.Sprintf("unable to find cluster by txID: %x", tx.Hash()))
		}

		if routed.Hash() == target.Hash() {
			return tx
		}
	}

	panic(fmt.Sprintf("unable to find transaction for target (%x) with %d clusters", target, len(clusters)))
}

// ClusterAssignment creates an assignment list with n clusters and with nodes
// evenly distributed among clusters.
func ClusterAssignment(n uint, nodes flow.IdentitySkeletonList) flow.AssignmentList {

	collectors := nodes.Filter(filter.HasRole[flow.IdentitySkeleton](flow.RoleCollection))

	// order, so the same list results in the same
	slices.SortFunc(collectors, flow.Canonical[flow.IdentitySkeleton])

	assignments := make(flow.AssignmentList, n)
	for i, collector := range collectors {
		index := uint(i) % n
		assignments[index] = append(assignments[index], collector.NodeID)
	}

	return assignments
}

func ClusterList(n uint, nodes flow.IdentitySkeletonList) flow.ClusterList {
	assignments := ClusterAssignment(n, nodes)
	clusters, err := factory.NewClusterList(assignments, nodes.Filter(filter.HasRole[flow.IdentitySkeleton](flow.RoleCollection)))
	if err != nil {
		panic(err)
	}

	return clusters
}

// CollectionFromTransactions creates a new collection from the list of transactions.
func CollectionFromTransactions(transactions ...*flow.TransactionBody) flow.Collection {
	txs := append(([]*flow.TransactionBody)(nil), transactions...) // copy slice to avoid mutation
	return flow.Collection{Transactions: txs}
}
