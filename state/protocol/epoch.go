package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// EpochQuery defines the different ways to query for epoch information
// given a Snapshot. It only exists to simplify the main Snapshot interface.
type EpochQuery interface {

	// Current returns the current epoch as of this snapshot. All valid snapshots
	// have a current epoch.
	// Error returns:
	//   - [state.ErrUnknownSnapshotReference] - if the epoch is queried from an unresolvable snapshot.
	//   - generic error in case of unexpected critical internal corruption or bugs
	Current() (CommittedEpoch, error)

	// NextUnsafe should only be used by components that are actively involved in advancing
	// the epoch from [flow.EpochPhaseSetup] to [flow.EpochPhaseCommitted].
	// NextUnsafe returns the tentative configuration for the next epoch as of this snapshot.
	// Valid snapshots make such configuration available during the Epoch Setup Phase, which
	// generally is the case only after an `EpochSetupPhaseStarted` notification has been emitted.
	// CAUTION: epoch transition might not happen as described by the tentative configuration!
	//
	// Error returns:
	//   - [ErrNextEpochNotSetup] in the case that this method is queried w.r.t. a snapshot
	//     within the [flow.EpochPhaseStaking] phase or when we are in Epoch Fallback Mode.
	//   - [ErrNextEpochAlreadyCommitted] if the tentative epoch is requested from
	//     a snapshot within the [flow.EpochPhaseCommitted] phase.
	//   - [state.ErrUnknownSnapshotReference] if the epoch is queried from an unresolvable snapshot.
	//   - generic error in case of unexpected critical internal corruption or bugs
	NextUnsafe() (TentativeEpoch, error)

	// NextCommitted returns the next epoch as of this snapshot, only if it has
	// been committed already - generally that is the case only after an
	// `EpochCommittedPhaseStarted` notification has been emitted.
	//
	// Error returns:
	//   - [ErrNextEpochNotCommitted] - in the case that committed epoch has been requested w.r.t a snapshot within
	//     the [flow.EpochPhaseStaking] or [flow.EpochPhaseSetup] phases.
	//   - [state.ErrUnknownSnapshotReference] - if the epoch is queried from an unresolvable snapshot.
	//   - generic error in case of unexpected critical internal corruption or bugs
	NextCommitted() (CommittedEpoch, error)

	// Previous returns the previous epoch as of this snapshot. Valid snapshots
	// must have a previous epoch for all epochs except that immediately after
	// the root block - in other words, if a previous epoch exists, implementations
	// must arrange to expose it here.
	//
	// Error returns:
	//   - [protocol.ErrNoPreviousEpoch] - if the epoch represents a previous epoch which does not exist.
	//     This happens when the previous epoch is queried within the first epoch of a spork.
	//   - [state.ErrUnknownSnapshotReference] - if the epoch is queried from an unresolvable snapshot.
	//   - generic error in case of unexpected critical internal corruption or bugs
	Previous() (CommittedEpoch, error)
}

// CommittedEpoch contains the information specific to a certain Epoch (defined
// by the epoch Counter). Note that the Epoch preparation can differ along
// different forks, since the emission of service events is fork-dependent.
// Therefore, an epoch exists RELATIVE to the snapshot from which it was
// queried.
//
// CAUTION: Clients must ensure to query epochs only for finalized blocks to
// ensure they query finalized epoch information.
//
// A CommittedEpoch instance is constant and reports the identical information
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
type CommittedEpoch interface {
	TentativeEpoch

	// FirstView returns the first view of this epoch.
	FirstView() uint64

	// DKGPhase1FinalView returns the final view of DKG phase 1
	DKGPhase1FinalView() uint64

	// DKGPhase2FinalView returns the final view of DKG phase 2
	DKGPhase2FinalView() uint64

	// DKGPhase3FinalView returns the final view of DKG phase 3
	DKGPhase3FinalView() uint64

	// FinalView returns the largest view number which still belongs to this epoch.
	// The largest view number is the greatest of:
	//   - the FinalView field of the flow.EpochSetup event for this epoch
	//   - the FinalView field of the most recent flow.EpochExtension for this epoch
	// If EFM is not triggered during this epoch, this value will be static.
	// If EFM is triggered during this epoch, this value may increase with increasing
	// reference block heights, as new epoch extensions are included.
	FinalView() uint64

	// TargetDuration returns the desired real-world duration for this epoch, in seconds.
	// This target is specified by the FlowEpoch smart contract along the TargetEndTime in
	// the EpochSetup event and used by the Cruise Control system to moderate the block rate.
	TargetDuration() uint64

	// TargetEndTime returns the desired real-world end time for this epoch, represented as
	// Unix Time (in units of seconds). This target is specified by the FlowEpoch smart contract in
	// the EpochSetup event and used by the Cruise Control system to moderate the block rate.
	TargetEndTime() uint64

	// RandomSource returns the underlying random source of this epoch.
	// This source is currently generated by an on-chain contract using the
	// UnsafeRandom() Cadence function.
	RandomSource() []byte

	// Cluster returns the detailed cluster information for the cluster with the
	// given index, in this epoch.
	// Error returns:
	//   * protocol.ErrClusterNotFound - if no cluster has the given index (index >= len(clusters))
	//   * generic error in case of internal state corruption
	Cluster(index uint) (Cluster, error)

	// ClusterByChainID returns the detailed cluster information for the cluster with
	// the given chain ID, in this epoch
	// Error returns:
	//   * protocol.ErrClusterNotFound - if cluster is not found by the given chainID
	//   * generic error in case of internal state corruption
	ClusterByChainID(chainID flow.ChainID) (Cluster, error)

	// DKG returns the result of the distributed key generation procedure.
	// No errors expected during normal operation.
	DKG() (DKG, error)

	// FirstHeight returns the height of the first block of the epoch.
	// The first block of an epoch E is defined as the block B with the lowest
	// height so that: B.View >= E.FirstView
	// The first block of an epoch is not defined until it is finalized.
	// Error returns:
	//   * protocol.ErrUnknownEpochBoundary - if the first block of the epoch is unknown or unfinalized.
	FirstHeight() (uint64, error)

	// FinalHeight returns the height of the final block of the epoch.
	// The final block of an epoch E is defined as the parent of the first
	// block in epoch E+1 (see definition from FirstHeight).
	// The final block of an epoch is not defined until its child is finalized.
	// Error returns:
	//   * protocol.ErrUnknownEpochBoundary - if the first block of the next epoch is unknown or unfinalized.
	FinalHeight() (uint64, error)
}

// TentativeEpoch returns the tentative information about the upcoming epoch,
// which the protocol is in the process of configuring.
// Only the data that is strictly necessary for committing the epoch is exposed;
// after commitment, all epoch data is accessible through the [CommittedEpoch] interface.
// This should only be used during the Epoch Setup Phase by components that actively
// contribute to configuring the upcoming epoch.
//
// CAUTION: the epoch transition might not happen as described by the tentative configuration!
type TentativeEpoch interface {

	// Counter returns the Epoch's counter.
	Counter() uint64

	// InitialIdentities returns the identities for this epoch as they were
	// specified in the EpochSetup service event.
	InitialIdentities() flow.IdentitySkeletonList

	// Clustering returns the cluster assignment for this epoch.
	// No errors expected during normal operation.
	Clustering() (flow.ClusterList, error)
}
