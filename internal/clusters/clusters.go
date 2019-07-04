// Package clusters implements the logic required to compute and validate access node clusters.
package clusters

import "github.com/dapperlabs/bamboo-node/pkg/crypto"

// Cluster is a grouping of access nodes.
type Cluster struct {
	Index uint64
	Nodes []crypto.Address
}

// Hash returns the unique hash of a cluster.
func (c *Cluster) Hash() crypto.Hash {
	return crypto.ZeroHash()
}

// ClusterManager is a utility to compute cluster arrangements.
type ClusterManager struct{}

// GetCluster returns the cluster that the given node belongs to.
func (c *ClusterManager) GetCluster(address crypto.Address, epoch uint64) (*Cluster, error) {
	return nil, nil
}

// GetClusters returns all clusters for a given epoch.
func (c *ClusterManager) GetClusters(epoch uint64) ([]*Cluster, error) {
	return nil, nil
}
