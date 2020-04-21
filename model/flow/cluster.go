package flow

import (
	"math/big"
)

// ClusterList is a set of clusters, keyed by ID. Each cluster must contain at
// least one node. All nodes are collection nodes.
type ClusterList struct {
	clusters []IdentityList
	lookup   map[Identifier]uint
}

// NewClusterList creates a new list of clusters.
func NewClusterList(nClusters uint) *ClusterList {
	cl := &ClusterList{
		clusters: make([]IdentityList, nClusters),
		lookup:   make(map[Identifier]uint),
	}
	return cl
}

// Add will add a node to the cluster list.
func (cl *ClusterList) Add(index uint, identity *Identity) {
	cl.clusters[int(index)] = append(cl.clusters[int(index)], identity)
	cl.lookup[identity.NodeID] = index
}

// ByIndex returns a cluster by index.
func (cl *ClusterList) ByIndex(index uint) IdentityList {
	return cl.clusters[int(index)]
}

// ByTxID selects the cluster that should receive the transaction with the given
// transaction ID.
//
// For evenly distributed transaction IDs, this will evenly distribute
// transactions between clusters.
func (cl *ClusterList) ByTxID(txID Identifier) IdentityList {
	bigTxID := new(big.Int).SetBytes(txID[:])
	bigIndex := new(big.Int).Mod(bigTxID, big.NewInt(int64(len(cl.clusters))))
	return cl.ByIndex(uint(bigIndex.Uint64()))
}

// ByNodeID select the cluster that the node with the given ID is part of.
//
// Nodes will be divided into equally sized clusters as far as possible.
func (cl ClusterList) ByNodeID(nodeID Identifier) (IdentityList, bool) {
	index, ok := cl.lookup[nodeID]
	if !ok {
		return nil, false
	}
	return cl.ByIndex(index), true
}

// Size returns the number of clusters.
func (cl ClusterList) Size() int {
	return len(cl.clusters)
}

// All returns all the clusters.
func (cl ClusterList) All() []IdentityList {
	return cl.clusters
}
