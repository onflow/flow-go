package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// EpochQuery defines the different ways to query for epoch information
// given a Snapshot. It only exists to simplify the main Snapshot interface.
type EpochQuery interface {

	// Current returns the current epoch as of this snapshot.
	Current() Epoch

	// Next returns the next epoch as of this snapshot.
	Next() Epoch

	// ByCounter returns an arbitrary epoch by counter.
	ByCounter(counter uint64) Epoch
}

// Epoch contains the information specific to a certain Epoch (defined
// by the epoch Counter). Note that the Epoch preparation can differ along
// different forks, since the emission of service events is fork-dependent.
// Therefore, an epoch exists RELATIVE to the snapshot from which it was
// queried.
//
// CAUTION: Clients must ensure to query epochs only for finalized blocks to
// ensure they query finalized epoch information.
//
// An Epoch instance is constant and reports the identical information
// even if progress is made later and more information becomes available in
// subsequent blocks.
//
// Methods error if epoch preparation has not progressed far enough for
// this information to be determined by a finalized block.
type Epoch interface {

	// Counter returns the Epoch's counter.
	Counter() (uint64, error)

	// FinalView returns the largest view number which still belongs to this epoch.
	FinalView() (uint64, error)

	// Seed generates a random seed using the source of randomness for this
	// epoch, specified in the EpochSetup service event.
	Seed(indices ...uint32) ([]byte, error)

	// InitialIdentities returns the identities for this epoch as they were
	// specified in the EpochSetup service event.
	InitialIdentities() (flow.IdentityList, error)

	// Clustering returns the cluster assignment for this epoch.
	// CAUTION: only clusters that operate from this particular Epoch are considered.
	Clusters() (ClusterList, error)

	// DKG returns the result of the distributed key generation procedure.
	DKG() (DKG, error)
}

// ClusterList is a list of clusters.
type ClusterList []Cluster

// ByIndex retrieves the list of identities that are part of the
// given cluster.
func (cl ClusterList) ByIndex(index uint) (Cluster, bool) {
	panic("Implement me")
}

// ByTxID selects the cluster that should receive the transaction with the given
// transaction ID.
//
// For evenly distributed transaction IDs, this will evenly distribute
// transactions between clusters.
func (cl ClusterList) ByTxID(txID flow.Identifier) (Cluster, bool) {
	panic("Implement me")
}

// ByNodeID selects the cluster that the node with the given ID is part of.
//
// Nodes will be divided into equally sized clusters as far as possible.
// The last return value will indicate if the look up was successful
func (cl ClusterList) ByNodeID(nodeID flow.Identifier) (Cluster, bool) {
	panic("Implement me")
}

// ByClusterID retruns the cluster with the given ID.
func (cl ClusterList) ByClusterID(clusterID flow.Identifier) (Cluster, bool) {
	panic("Implement me")
}
