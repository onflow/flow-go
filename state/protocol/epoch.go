package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// EpochQuery defines the different ways to query for epoch information
// given a Snapshot. It only exists to simplify the main Snapshot interface.
type EpochQuery interface {

	// Current returns the current epoch as of this snapshot. All valid snapshots
	// have a current epoch.
	Current() Epoch

	// Next returns the next epoch as of this snapshot. Valid snapshots must
	// have a next epoch available after the transition to epoch setup phase.
	Next() Epoch

	// Previous returns the previous epoch as of this snapshot. Valid snapshots
	// must have a previous epoch for all epochs except that immediately after
	// the root block - in other words, if a previous epoch exists, implementations
	// must arrange to expose it here.
	//
	// Returns ErrNoPreviousEpoch in the case that this method is queried w.r.t.
	// a snapshot from the first epoch after the root block.
	Previous() Epoch
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
//
// TODO Epoch / Snapshot API Structure:  Currently Epoch and Snapshot APIs
// are structured to allow chained queries to be used without error checking
// at each call where errors might occur. Instead, errors are cached in the
// resulting struct (eg. invalid.Epoch) until the query chain ends with a
// function which can return an error. This has some negative effects:
//  1. Cached intermediary errors result in more complex error handling
//     a) each final call of the chained query needs to handle all intermediary errors, every time
//     b) intermediary errors must be handled by dependencies on the final call of the query chain (eg. conversion functions)
//  2. The error caching pattern encourages potentially dangerous snapshot query patterns
//
// See https://github.com/dapperlabs/flow-go/issues/6368 for details and proposal
type Epoch interface {

	// Counter returns the Epoch's counter.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	Counter() (uint64, error)

	// FirstView returns the first view of this epoch.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	FirstView() (uint64, error)

	// DKGPhase1FinalView returns the final view of DKG phase 1
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	DKGPhase1FinalView() (uint64, error)

	// DKGPhase2FinalView returns the final view of DKG phase 2
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	DKGPhase2FinalView() (uint64, error)

	// DKGPhase3FinalView returns the final view of DKG phase 3
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	DKGPhase3FinalView() (uint64, error)

	// FinalView returns the largest view number which still belongs to this epoch.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	FinalView() (uint64, error)

	// TargetDuration returns the desired real-world duration for this epoch, in seconds.
	// This target is specified by the FlowEpoch smart contract along the TargetEndTime in
	// the EpochSetup event and used by the Cruise Control system to moderate the block rate.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	TargetDuration() (uint64, error)

	// TargetEndTime returns the desired real-world end time for this epoch, represented as
	// Unix Time (in units of seconds). This target is specified by the FlowEpoch smart contract in
	// the EpochSetup event and used by the Cruise Control system to moderate the block rate.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	TargetEndTime() (uint64, error)

	// RandomSource returns the underlying random source of this epoch.
	// This source is currently generated by an on-chain contract using the
	// UnsafeRandom() Cadence function.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	RandomSource() ([]byte, error)

	// InitialIdentities returns the identities for this epoch as they were
	// specified in the EpochSetup service event.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	InitialIdentities() (flow.IdentitySkeletonList, error)

	// Clustering returns the cluster assignment for this epoch.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	Clustering() (flow.ClusterList, error)

	// Cluster returns the detailed cluster information for the cluster with the
	// given index, in this epoch.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	// * protocol.ErrClusterNotFound - if no cluster has the given index (index > len(clusters))
	Cluster(index uint) (Cluster, error)

	// ClusterByChainID returns the detailed cluster information for the cluster with
	// the given chain ID, in this epoch
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	// * protocol.ErrNextEpochNotCommitted - if epoch has not been committed yet
	// * protocol.ErrClusterNotFound - if cluster is not found by the given chainID
	ClusterByChainID(chainID flow.ChainID) (Cluster, error)

	// DKG returns the result of the distributed key generation procedure.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * protocol.ErrNextEpochNotCommitted if epoch has not been committed yet
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	DKG() (DKG, error)

	// FirstHeight returns the height of the first block of the epoch.
	// The first block of an epoch E is defined as the block B with the lowest
	// height so that: B.View >= E.FirstView
	// The first block of an epoch is not defined until it is finalized, so this
	// value is only guaranteed to be defined for `Current` epochs of finalized snapshots.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * protocol.ErrNextEpochNotCommitted if epoch has not been committed yet
	// * protocol.ErrEpochTransitionNotFinalized - if the first block of the epoch has not been finalized yet.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	FirstHeight() (uint64, error)

	// FinalHeight returns the height of the final block of the epoch.
	// The final block of an epoch E is defined as the parent of the first
	// block in epoch E+1 (see definition from FirstHeight).
	// The final block of an epoch is not defined until its child is finalized,
	// so this value is only guaranteed to be defined for `Previous` epochs of finalized snapshots.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * protocol.ErrNextEpochNotCommitted - if epoch has not been committed yet
	// * protocol.ErrEpochTransitionNotFinalized - if the first block of the next epoch has not been finalized yet.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	FinalHeight() (uint64, error)
}
