package protocol

import (
	"math/big"
	"sort"

	"github.com/dapperlabs/flow-go/crypto"

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

// Route routes a transaction to a cluster given its hash and the number of
// clusters in the system.
//
// For evenly distributed transaction hashes, this will evenly distribute
// transaction between clusters.
func Route(nClusters int, txHash crypto.Hash) flow.ClusterID {
	if nClusters < 1 {
		return flow.ClusterID(0)
	}

	txHashBigInt := new(big.Int).SetBytes(txHash[:])

	clusterIDBigInt := new(big.Int).Mod(txHashBigInt, big.NewInt(int64(nClusters)))
	clusterID := flow.ClusterID(clusterIDBigInt.Uint64())

	if clusterID >= flow.ClusterID(nClusters) {
		panic("routed to invalid cluster") // should never happen
	}

	return clusterID
}
