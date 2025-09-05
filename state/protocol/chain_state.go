package protocol

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// State represents the full protocol state of the local node. It allows us to
// obtain snapshots of the state at any point of the protocol state history.
type State interface {

	// Params gives access to a number of stable parameters of the protocol state.
	Params() Params

	// Final returns the snapshot of the persistent protocol state at the latest
	// finalized block, and the returned snapshot is therefore immutable over
	// time.
	Final() Snapshot

	// Sealed returns the snapshot of the persistent protocol state at the
	// latest sealed block, and the returned snapshot is therefore immutable
	// over time.
	Sealed() Snapshot

	// AtHeight returns the snapshot of the persistent protocol state at the
	// given block number. It is only available for finalized blocks and the
	// returned snapshot is therefore immutable over time.
	AtHeight(height uint64) Snapshot

	// AtBlockID returns the snapshot of the persistent protocol state at the
	// given block ID. It is available for any block that was introduced into
	// the protocol state, and can thus represent an ambiguous state that was or
	// will never be finalized.
	AtBlockID(blockID flow.Identifier) Snapshot
}

// FollowerState is a mutable protocol state used by nodes following main consensus (ie. non-consensus nodes).
// All blocks must have a certifying QC when being added to the state to guarantee they are valid,
// so there is a one-block lag between block production and incorporation into the FollowerState.
// However, since all blocks are certified upon insertion, they are immediately processable by other components.
type FollowerState interface {
	State

	// ExtendCertified introduces the block with the given ID into the persistent
	// protocol state without modifying the current finalized state. It allows us
	// to execute fork-aware queries against the known protocol state. As part of
	// the CertifiedBlock, the caller must pass a Quorum Certificate [QC] (field
	// `CertifyingQC`) to prove that the candidate block has been certified, and
	// it's safe to add to it the protocol state. The QC cannot be nil and must
	// certify candidate block:
	//   candidate.View == QC.View && candidate.BlockID == QC.BlockID
	//
	// CAUTION:
	//   - This function expects that `QC` has been validated. (otherwise, the state will be corrupted)
	//   - The parent block must already be stored.
	//   - Attempts to extend the state with the _same block concurrently_ are not allowed.
	//     (will not corrupt the state, but may lead to an exception)
	//
	// Aside from the requirement that ancestors must have been previously ingested, all blocks are
	// accepted; no matter how old they are; or whether they are orphaned or not.
	//
	// Note: To ensure that all ancestors of a candidate block are correct and known to the FollowerState, some external
	// ordering and queuing of incoming blocks is generally necessary (responsibility of Compliance Layer). Once a block
	// is successfully ingested, repeated extension requests with this block are no-ops. This is convenient for the
	// Compliance Layer after a crash, so it doesn't have to worry about which blocks have already been ingested before
	// the crash. However, while running it is very easy for the Compliance Layer to avoid concurrent extension requests
	// with the same block. Hence, for simplicity, the FollowerState may reject such requests with an exception.
	//
	// No errors are expected during normal operations.
	//   - In case of concurrent calls with the same `candidate` block, `ExtendCertified` may return a [storage.ErrAlreadyExists]
	//     or it may gracefully return. At the moment, `ExtendCertified` should be considered as NOT CONCURRENCY-SAFE.
	ExtendCertified(ctx context.Context, certified *flow.CertifiedBlock) error

	// Finalize finalizes the block with the given hash.
	// At this level, we can only finalize one block at a time. This implies
	// that the parent of the pending block that is to be finalized has
	// to be the last finalized block.
	// It modifies the persistent immutable protocol state accordingly and
	// forwards the pointer to the latest finalized state.
	// No errors are expected during normal operations.
	Finalize(ctx context.Context, blockID flow.Identifier) error
}

// ParticipantState is a mutable protocol state used by active consensus participants (consensus nodes).
// All blocks are validated in full, including payload validation, prior to insertion. Only valid blocks are inserted.
type ParticipantState interface {
	FollowerState

	// Extend introduces the block with the given ID into the persistent
	// protocol state without modifying the current finalized state. It allows
	// us to execute fork-aware queries against ambiguous protocol state, while
	// still checking that the given block is a valid extension of the protocol state.
	// The candidate block must have passed HotStuff validation before being passed to Extend.
	//
	// CAUTION:
	//   - per convention, the protocol state requires that the candidate's
	//     parent has already been ingested. Otherwise, an exception is returned.
	//   - Attempts to extend the state with the _same block concurrently_ are not allowed.
	//     (will not corrupt the state, but may lead to an exception)
	//   - We reject orphaned blocks with [state.OutdatedExtensionError] !
	//     This is more performant, but requires careful handling by the calling code. Specifically,
	//     the caller should not just drop orphaned blocks from the cache to avoid wasteful re-requests.
	//     If we were to entirely forget orphaned blocks, e.g. block X of the orphaned fork X ← Y ← Z,
	//     we might not have enough information to reject blocks Y, Z later if we receive them. We would
	//     re-request X, then determine it is orphaned and drop it, attempt to ingest Y re-request the
	//     unknown parent X and repeat potentially very often.
	//
	// Note: To ensure that all ancestors of a candidate block are correct and known to the Protocol State, some external
	// ordering and queuing of incoming blocks is generally necessary (responsibility of Compliance Layer). Once a block
	// is successfully ingested, repeated extension requests with this block are no-ops. This is convenient for the
	// Compliance Layer after a crash, so it doesn't have to worry about which blocks have already been ingested before
	// the crash. However, while running it is very easy for the Compliance Layer to avoid concurrent extension requests
	// with the same block. Hence, for simplicity, the FollowerState may reject such requests with an exception.
	//
	// Expected errors during normal operations:
	//  * [state.OutdatedExtensionError] if the candidate block is orphaned
	//  * [state.InvalidExtensionError] if the candidate block is invalid
	//  * In case of concurrent calls with the same `candidate` block, `Extend` may return a [storage.ErrAlreadyExists]
	//    or it may gracefully return. At the moment, `Extend` should be considered as NOT CONCURRENCY-SAFE.
	Extend(ctx context.Context, candidate *flow.Proposal) error
}
