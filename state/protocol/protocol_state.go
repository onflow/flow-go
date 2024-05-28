package protocol

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// EpochProtocolState represents the subset of the Protocol State KVStore related to epochs:
// the Identity Table, DKG, cluster assignment, etc.
// EpochProtocolState is fork-aware and can change on a block-by-block basis.
// Each EpochProtocolState instance refers to the state with respect to some reference block.
type EpochProtocolState interface {
	// Epoch returns the current epoch counter.
	Epoch() uint64
	// Clustering returns initial clustering from epoch setup.
	// No errors are expected during normal operations.
	Clustering() (flow.ClusterList, error)
	// EpochSetup returns original epoch setup event that was used to initialize the protocol state.
	EpochSetup() *flow.EpochSetup
	// EpochCommit returns original epoch commit event that was used to update the protocol state.
	EpochCommit() *flow.EpochCommit
	// DKG returns information about DKG that was obtained from EpochCommit event.
	// No errors are expected during normal operations.
	DKG() (DKG, error)
	// InvalidEpochTransitionAttempted denotes whether an invalid epoch state transition was attempted
	// on the fork ending this block. Once the first block where this flag is true is finalized, epoch
	// fallback mode is triggered.
	// TODO for 'leaving Epoch Fallback via special service event': at the moment, this is a one-way transition and requires a spork to recover - need to revisit for sporkless EFM recovery
	InvalidEpochTransitionAttempted() bool
	// PreviousEpochExists returns true if a previous epoch exists. This is true for all epoch
	// except those immediately following a spork.
	PreviousEpochExists() bool
	// EpochPhase returns the epoch phase for the current epoch.
	EpochPhase() flow.EpochPhase

	// Identities returns identities (in canonical ordering) that can participate in the current or previous
	// or next epochs. Let P be the set of identities in the previous epoch, C be the set of identities in
	// the current epoch, and N be the set of identities in the next epoch.
	// The set of authorized identities this function returns is different depending on epoch state:
	// EpochStaking phase:
	//   - nodes in C with status `flow.EpochParticipationStatusActive`
	//   - nodes in P-C with status `flow.EpochParticipationStatusLeaving`
	// EpochSetup/EpochCommitted phase:
	//   - nodes in C with status `flow.EpochParticipationStatusActive`
	//   - nodes in N-C with status `flow.EpochParticipationStatusJoining`
	Identities() flow.IdentityList

	// GlobalParams returns global, static network params that are same for all nodes in the network.
	GlobalParams() GlobalParams

	// Entry returns low-level protocol state entry that was used to initialize this object.
	// It shouldn't be used by high-level logic, it is useful for some cases such as bootstrapping.
	// Prefer using other methods to access protocol state.
	Entry() *flow.RichEpochProtocolStateEntry
}

// ProtocolState is the read-only interface for protocol state. It allows querying the
// Protocol KVStore or Epoch sub-state by block, and retrieving global network params.
type ProtocolState interface {
	// EpochStateAtBlockID returns epoch protocol state at block ID.
	// The resulting epoch protocol state is returned AFTER applying updates that are contained in block.
	// Can be queried for any block that has been added to the block tree.
	// Returns:
	// - (EpochProtocolState, nil) - if there is an epoch protocol state associated with given block ID.
	// - (nil, storage.ErrNotFound) - if there is no epoch protocol state associated with given block ID.
	// - (nil, exception) - any other error should be treated as exception.
	EpochStateAtBlockID(blockID flow.Identifier) (EpochProtocolState, error)

	// KVStoreAtBlockID returns protocol state at block ID.
	// The resulting protocol state is returned AFTER applying updates that are contained in block.
	// Can be queried for any block that has been added to the block tree.
	// Returns:
	// - (KVStoreReader, nil) - if there is a protocol state associated with given block ID.
	// - (nil, storage.ErrNotFound) - if there is no protocol state associated with given block ID.
	// - (nil, exception) - any other error should be treated as exception.
	KVStoreAtBlockID(blockID flow.Identifier) (KVStoreReader, error)

	// GlobalParams returns params that are the same for all nodes in the network.
	GlobalParams() GlobalParams
}

// MutableProtocolState is the read-write interface for protocol state. It allows evolving the protocol
// state by calling `EvolveState` for each block with arguments that might trigger state changes.
type MutableProtocolState interface {
	ProtocolState

	// EvolveState updates the overall Protocol State based on information in the candidate block
	// (potentially still under construction). Information that may change the state is:
	//   - the candidate block's view
	//   - Service Events from execution results sealed in the candidate block
	//
	// EvolveState is compatible with speculative processing: it evolves an *in-memory copy* of the parent state
	// and collects *deferred database updates* for persisting the resulting Protocol State, including all of its
	// dependencies and respective indices. Though, the resulting batch of deferred database updates still depends
	// on the candidate block's ID, which is still unknown at the time of block construction. Executing the deferred
	// database updates is the caller's responsibility.
	//
	// SAFETY REQUIREMENTS:
	//  1. The seals must be a protocol-compliant extension of the parent block. Intuitively, we require that the
	//     seals follow the ancestry of this fork without gaps. The Consensus Participant's Compliance Layer enforces
	//     the necessary constrains. Analogously, the block building logic should always produce protocol-compliant
	//     seals.
	//     The seals guarantee correctness of the sealed execution result, including the contained service events.
	//     This is actively checked by the verification node, whose aggregated approvals in the form of a seal attest
	//     to the correctness of the sealed execution result (specifically the Service Events contained in the result
	//     and their order).
	//  2. For Consensus Participants that are replicas, the calling code must check that the returned `stateID` matches
	//     the commitment in the block proposal! If they don't match, the proposer is byzantine and should be slashed.
	//
	// Consensus nodes actively verify protocol compliance for any block proposal they receive, including integrity of
	// each seal individually as well as the seals continuously following the fork. Light clients only process certified
	// blocks, which guarantees that consensus nodes already ran those checks and found the proposal to be valid.
	//
	// SERVICE EVENTS form an order-preserving, asynchronous, one-way message bus from the System Smart Contracts
	// (verified honest execution) to the Protocol State. For example, consider a fork where a service event is
	// emitted during execution of block A. Block B contains an execution receipt `RA` for A. Block C holds a
	// seal `SA` for A's execution result.
	//
	//    A ← … ← B(RA) ← … ← C(SA)
	//
	// Service Events are included within execution results, which are stored opaquely as part of the block payload
	// (block B in our example). Though, to ensure correctness of the service events, we only process them upon sealing.
	// There is some non-deterministic delay when the Protocol State observes the Service Events from block A.
	// In our example, any change to the protocol state requested by the system smart contracts in block A, would only
	// become visible in block C's Protocol State (and descendants).
	//
	// Error returns:
	// [TLDR] All error returns indicate potential state corruption and should therefore be treated as fatal.
	//   - Per convention, the input seals from the block payload have already been confirmed to be protocol compliant.
	//     Hence, the service events in the sealed execution results represent the honest execution path.
	//     Therefore, the sealed service events should encode a valid evolution of the protocol state -- provided
	//     the system smart contracts are correct.
	//   - As we can rule out byzantine attacks as the source of failures, the only remaining sources of problems
	//     can be (a) bugs in the system smart contracts or (b) bugs in the node implementation. A service event
	//     not representing a valid state transition despite all consistency checks passing is interpreted as
	//     case (a) and _should be_ handled internally by the respective state machine. Otherwise, any bug or
	//     unforeseen edge cases in the system smart contracts would result in consensus halt, due to errors while
	//     evolving the protocol state.
	//   - A consistency or sanity check failing within the StateMutator is likely the symptom of an internal bug
	//     in the node software or state corruption, i.e. case (b). This is the only scenario where the error return
	//     of this function is not nil. If such an exception is returned, continuing is not an option.
	EvolveState(parentBlockID flow.Identifier, candidateView uint64, candidateSeals []*flow.Seal) (stateID flow.Identifier, dbUpdates *transaction.DeferredBlockPersist, err error)
}
