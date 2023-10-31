package protocol

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// InitialProtocolState returns constant data for given epoch.
// This interface can be only obtained for epochs that have progressed to epoch commit event.
type InitialProtocolState interface {
	// Epoch returns counter of epoch.
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
	// Entry Returns low-level protocol state entry that was used to initialize this object.
	// It shouldn't be used by high-level logic, it is useful for some cases such as bootstrapping.
	// Prefer using other methods to access protocol state.
	Entry() *flow.RichProtocolStateEntry
}

// DynamicProtocolState extends the InitialProtocolState with data that can change from block to block.
// It can be used to access the identity table at given block.
type DynamicProtocolState interface {
	InitialProtocolState
	// EpochStatus returns the status of current epoch at given block based on the internal state of protocol.
	EpochStatus() *flow.EpochStatus
	// Identities returns identities that can participate in current and next epochs.
	// Set of Authorized identities are different depending on epoch state:
	// staking phase - identities for current epoch + identities from previous epoch (with 0 weight)
	// setup & commit phase - identities for current epoch + identities from next epoch (with 0 weight)
	Identities() flow.IdentityList
	// GlobalParams returns params that are same for all nodes in the network.
	GlobalParams() GlobalParams
}

// ProtocolState is the read-only interface for protocol state, it allows to query information
// on a per-block and per-epoch basis.
type ProtocolState interface {
	// ByEpoch returns an object with static protocol state information by epoch number.
	// To be able to use this interface we need to observe both epoch setup and commit events.
	// Not available for next epoch unless we have observed an EpochCommit event.
	// No errors are expected during normal operations.
	// TODO(yuraolex): check return types
	// TODO(yuraolex): decide if we really need this approach. It's unclear if it's useful to query
	//  by epoch counter. To implement it we need an additional index by epoch counter. Alternatively we need a way to map
	//  epoch counter -> block ID. It gets worse if we consider that we need a way to get the epoch counter itself at caller side.
	//ByEpoch(epoch uint64) (InitialProtocolState, error)

	// AtBlockID returns protocol state at block ID.
	// The resulting protocol state is returned AFTER applying updates that are contained in block.
	// Can be queried for any block that has been added to the block tree.
	// Returns:
	// - (DynamicProtocolState, nil) - if there is a protocol state associated with given block ID.
	// - (nil, storage.ErrNotFound) - if there is no protocol state associated with given block ID.
	// - (nil, exception) - any other error should be treated as exception.
	AtBlockID(blockID flow.Identifier) (DynamicProtocolState, error)
	// GlobalParams returns params that are the same for all nodes in the network.
	GlobalParams() GlobalParams
}

type MutableProtocolState interface {
	ProtocolState

	// Mutator instantiates a `StateMutator` based on the previous protocol state.
	// Has to be called for each block to evolve the protocol state.
	// Expected errors during normal operations:
	//  * `storage.ErrNotFound` if no protocol state for parent block is known.
	Mutator(candidateView uint64, parentID flow.Identifier) (StateMutator, error)
}

// StateMutator is a stateful object to evolve the protocol state. It is instantiated from the parent block's protocol state.
// State-changing operations can be iteratively applied and the StateMutator will internally evolve its in-memory state.
// While the StateMutator does not modify the database, it internally tracks all necessary updates. Upon calling `Build`
// the StateMutator returns the updated protocol state, its ID and all database updates necessary for persisting the updated
// protocol state.
// The StateMutator is used by a replica's compliance layer to update protocol state when observing state-changing service in
// blocks. It is used by the primary in the block building process to obtain the correct protocol state for a proposal.
// Specifically, the leader may include state-changing service events in the block payload. The flow protocol prescribes that
// the proposal needs to include the ID of the protocol state, _after_ processing the payload incl. all state-changing events.
// Therefore, the leader instantiates a StateMutator, applies the service events to it and builds the updated protocol state ID.
type StateMutator interface {
	// Build returns:
	//  - hasChanges: flag whether there were any changes; otherwise, `updatedState` and `stateID` equal the parent state
	//  - updatedState: the ProtocolState after applying all updates
	//  - stateID: the has commitment to the `updatedState`
	//  - dbUpdates: database updates necessary for persisting the updated protocol state
	// updated protocol state entry, state ID and a flag indicating if there were any changes.
	Build() (hasChanges bool, updatedState *flow.ProtocolStateEntry, stateID flow.Identifier, dbUpdates []func(tx *transaction.Tx) error, err error)

	// ApplyServiceEvents handles applying state changes which occur as a result
	// of service events being included in a block payload.
	// All updates that are incorporated in service events are applied to the protocol state by mutating protocol state updater.
	// No errors are expected during normal operation.
	ApplyServiceEvents(seals []*flow.Seal) error
}
