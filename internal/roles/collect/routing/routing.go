// Package routing implements a deterministic transaction routing algorithm.
package routing

import (
	"github.com/dapperlabs/flow-go/internal/roles/collect/clusters"
	"github.com/dapperlabs/flow-go/pkg/types"
)

// TransactionRouter routes transactions to clusters using a deterministic transaction routing algorithm.
type TransactionRouter struct {
	clusterManager *clusters.ClusterManager
}

// Route returns the routed cluster for a transaction in the given epoch.
func (tr *TransactionRouter) Route(transaction *types.Transaction, epoch uint64) *clusters.Cluster {
	return nil
}

// InRange returns true if the transaction belongs in the given cluster, and false otherwise.
func (tr *TransactionRouter) InRange(transaction *types.Transaction, cluster *clusters.Cluster) bool {
	return false
}
