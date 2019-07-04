// Package routing implements a deterministic transaction routing algorithm.
package routing

import (
	"github.com/dapperlabs/bamboo-node/internal/clusters"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// TransactionRouter routes transactions to clusters using a deterministic transaction routing algorithm.
type TransactionRouter struct {
	clusterManager *clusters.ClusterManager
}

// Route returns the routed clusters for a transaction in the given epoch.
func (tr *TransactionRouter) Route(transaction *types.SignedTransaction, epoch uint64) *clusters.Cluster {
	return nil
}

// InRange returns true if the transaction belongs in the given cluster, and false otherwise.
func (tr *TransactionRouter) InRange(transaction *types.SignedTransaction, cluster *clusters.Cluster) bool {
	return false
}
