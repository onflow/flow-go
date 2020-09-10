package protocol

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

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

	// FinalView returns the largest view number which still belongs to this Epoch.
	FinalView() (uint64, error)

	// LeaderSelectionSeed returns the seed for leader selection for this epoch
	// using the source of randomness specified in the EpochSetup service event.
	LeaderSelectionSeed(indices ...uint32) ([]byte, error)

	// InitialIdentities returns the identities for this epoch as they were
	// specified in the EpochSetup service event.
	InitialIdentities() (flow.IdentityList, error)

	// Clustering returns the cluster assignment for this epoch.
	Clustering() (flow.ClusterList, error)

	// Cluster returns the detailed cluster information for the cluster with the
	// given index, in this epoch.
	Cluster(index uint32) (Cluster, error)

	// DKG returns the result of the distributed key generation procedure.
	DKG() (DKG, error)
}
