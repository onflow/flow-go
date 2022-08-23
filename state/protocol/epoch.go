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
// TODO Epoch / Snapshot API Structure:
//
// Currently Epoch and Snapshot APIs are structured to allow chained queries
// to be used without error checking at each call where errors might occur.
// Instead, errors are cached in the resulting struct (eg. invalid.Epoch)
// until the query chain ends with a function which can return an error.
//
// For example, snapshot.Epochs().Next() could introduce an error ErrNextEpochNotSetup
// if the next epoch is not set up. However, the error is not returned and
// therefore cannot be checked until we query something about the resulting epoch.
//
// This has the negative side-effect of expanding the surface error of errors
// introduced in an intermediary chained call. If the error were returned when
// introduced, it could be checked once, then the Epoch could be queried safely.
// However, since the errors introduced at snapshot.Epochs().Next() are not
// exposed until the Epoch is queried, to conform with error handling guidelines
// we need to check them for each query on Epoch, even though we should only need
// to check them once.
//
// CURRENT ERROR CHECKING REQUIREMENT:
//
// counter, err := snapshot.Epochs().Next().Counter()
// if errors.Is(err, ErrNoPreviousEpoch) {...}
// if errors.Is(err, ErrNextEpochNotSetup) {...}
// if errors.Is(err, state.ErrUnknownSnapshotReference) {...}
// if err != nil {...}
//
// dkg, err := snapshot.Epochs().Next().DKG()
// if errors.Is(err, ErrNoPreviousEpoch) {...}
// if errors.Is(err, ErrNextEpochNotSetup) {...}
// if errors.Is(err, state.ErrUnknownSnapshotReference) {...}
// if errors.Is(err, ErrEpochNotCommitted) {...}
// if err != nil {...}
//
// ERROR CHECKING REQUIREMENTS WITH INTERMEDIARY CALLS RETURNING ERRORS:
//
// epoch, err := snapshot.Epochs().Next()
// if errors.Is(err, ErrNextEpochNotSetup) {...}
// if errors.Is(err, state.ErrUnknownSnapshotReference) {...}
// if err != nil {...}
//
// counter, err := epoch.Counter()
// if err != nil {...}
//
// dkg, err := snapshot.Epochs().Next().DKG()
// if errors.Is(err, ErrEpochNotCommitted) {...}
// if err != nil {...}
//
// The current pattern also unnecessarily expands the error handling scope of
// conversion functions. For example, inmem.FromEpoch must handle, and might
// return any of:
// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
// even though these errors all occur when the epoch is queried, and could be
// handled before the the conversion occurs.
//
// NOTE: these problems similarly affect Snapshot.
//
// For Snapshot in particular, this pattern causes another problem. When querying
// a protocol state snapshot, it is desirable to query exactly one snapshot reference
// (eg. state.Final())and use the same reference for the duration of the process.
// Querying state.Final()... multiple times in the same process can lead to subtle bugs,
// because the finalized block might change between calls.
// Forcing declaring an intermediary variable by error checking at the point the
// snapshot is defined encourages using the same snapshot, as desired.
//
// CURRENT PATTERN:
// head, err := state.Final().Head()
// if err != nil {...}
// qc, err := state.Final().QuorumCertificate() // we might query a different finalized snapshot here!
// if err != nil {...}
//
// ALTERNATIVE PATTERN:
// final, err := state.Final() // returning error encourages creating a local variable bound to a single consistent snapshot
// if err != nil {...}
//
// head, err := final.Head()
// if err != nil {...}
// qc, err := final.QuorumCertificate()
// if err != nil {...}
//
// SUMMARY:
// 1. Cached intermediary errors result in more complex error handling
//    a) each final call of the chained query needs to handle all intermediary errors, every time
//    b) intermediary errors must be handled by dependencies on the final call of the query chain (eg. conversion functions)
// 2. The error caching pattern encourages potentially dangerous snapshot query patterns
//
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
	InitialIdentities() (flow.IdentityList, error)

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
	Cluster(index uint) (Cluster, error)

	// ClusterByChainID returns the detailed cluster information for the cluster with
	// the given chain ID, in this epoch
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	// * protocol.ErrEpochNotCommitted if epoch has not been committed yet
	// * protocol.ErrClusterNotFound if cluster is not found by the given chainID
	ClusterByChainID(chainID flow.ChainID) (Cluster, error)

	// DKG returns the result of the distributed key generation procedure.
	// Error returns:
	// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
	// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
	// * protocol.ErrEpochNotCommitted if epoch has not been committed yet
	// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
	DKG() (DKG, error)
}
