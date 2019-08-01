// Package clusters implements the logic required to compute and validate access node clusters.
package clusters

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// Cluster is a grouping of access nodes.
type Cluster struct {
	Index uint64
	Nodes []types.Address
}

// Hash returns the unique hash of a cluster.
func (c *Cluster) Hash() crypto.Hash {
	return nil
}

// ClusterManager is a utility to compute cluster arrangements.
//
// Clusters are computed using the following algorithm:
// 1. For each staked node, compute the bitwise XOR between the node address and the epoch root hash.
// 2. Sort all nodes by the result of the XOR.
// 3. Split the list into N equal sized chunks, where N is the target number of clusters.
type ClusterManager struct{}

// GetCluster returns the cluster that the given node belongs to.
func (c *ClusterManager) GetCluster(address types.Address, epoch uint64) (*Cluster, error) {
	return nil, nil
}

// GetClusters returns all clusters for a given epoch.
func (c *ClusterManager) GetClusters(epoch uint64) ([]*Cluster, error) {
	return nil, nil
}
