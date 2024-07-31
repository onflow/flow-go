package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// This file contains versioned read-write interfaces to the Protocol State's
// key-value store and are used by the Protocol State Machine.
//
// When a key is added or removed, this requires a new protocol state version:
//  - Create a new versioned model in ./kvstore/models.go (eg. modelv3 if latest model is modelv2)
//  - Update the protocol.KVStoreReader and KVStoreAPI interfaces to include any new keys

// KVStoreAPI is the latest interface to the Protocol State key-value store which implements 'Prototype'
// pattern for replicating protocol state between versions.
//
// Caution:
// Engineers evolving this interface must ensure that it is backwards-compatible
// with all versions of Protocol State Snapshots that can be retrieved from the local
// database, which should exactly correspond to the versioned model types defined in
// ./kvstore/models.go
type KVStoreAPI interface {
	protocol.KVStoreReader

	// Replicate instantiates a Protocol State Snapshot of the given `protocolVersion`.
	// We reference to the Protocol State Snapshot, whose `Replicate` method is called
	// as the 'Parent Snapshot'.
	// If the `protocolVersion` matches the version of the Parent Snapshot, `Replicate` behaves
	// exactly like a deep copy. If `protocolVersion` is newer, the data model corresponding
	// to the newer version is used and values from the Parent Snapshot are replicated into
	// the new data model. In all cases, the new Snapshot can be mutated without changing the
	// Parent Snapshot.
	//
	// Caution:
	// Implementors of this function decide on their own how to perform the migration from parent protocol version
	// to the given `protocolVersion`. It is required that outcome of `Replicate` is a valid KV store model which can be
	// incorporated in the protocol state without extra operations.
	// Expected errors during normal operations:
	//  - kvstore.ErrIncompatibleVersionChange if replicating the Parent Snapshot into a Snapshot
	//    with the specified `protocolVersion` is not supported.
	Replicate(protocolVersion uint64) (KVStoreMutator, error)
}

// KVStoreMutator is the latest read-writer interface to the Protocol State key-value store.
//
// Caution:
// Engineers evolving this interface must ensure that it is backwards-compatible
// with all versions of Protocol State Snapshots that can be retrieved from the local
// database, which should exactly correspond to the versioned model types defined in
// ./kvstore/models.go
type KVStoreMutator interface {
	protocol.KVStoreReader

	// v0/v1

	// SetVersionUpgrade sets the protocol upgrade version. This method is used
	// to update the Protocol State version when a flow.ProtocolStateVersionUpgrade is processed.
	// It contains the new version and the view at which it has to be applied.
	SetVersionUpgrade(version *protocol.ViewBasedActivator[uint64])

	// SetEpochStateID sets the state ID of the epoch state.
	// This method is used to commit the epoch state to the KV store when the state of the epoch is updated.
	SetEpochStateID(stateID flow.Identifier)

	// SetEpochExtensionViewCount sets the number of views for a hypothetical epoch extension.
	// Expected errors during normal operations:
	//  - kvstore.ErrInvalidValue - if the view count is less than FinalizationSafetyThreshold*2.
	SetEpochExtensionViewCount(viewCount uint64) error
}

// OrthogonalStoreStateMachine represents a state machine that exclusively evolves its state P.
// The state's specific type P is kept as a generic. Generally, P is the type corresponding
// to one specific key in the Key-Value store.
//
// Orthogonal State Machines:
// Orthogonality means that state machines can operate completely independently and work on disjoint
// sub-states. By convention, they all consume the same inputs (incl. the ordered sequence of
// Service Events sealed in one block). In other words, each state machine has full visibility into
// the inputs, but each draws their on independent conclusions (maintain their own exclusive state).
//
// The Dynamic Protocol State comprises a Key-Value-Store. We loosely associate each key-value-pair
// with a dedicated state machine operating exclusively on this key-value pair. A one-to-one
// correspondence between key-value-pair and state machine should be the default, but is not strictly
// required. However, we strictly require that no key-value-pair is being operated on by *more* than
// one state machine.
//
// The Protocol State is the framework, which orchestrates the orthogonal state machines, feeds them
// with inputs, post-processes the outputs and overall manages state machines' life-cycle from block
// to block. New key-value pairs and corresponding state machines can easily be added by
//   - adding a new entry to the Key-Value-Store's data model (file `./kvstore/models.go`)
//   - implementing the `OrthogonalStoreStateMachine` interface
//
// For more details see `./Readme.md`
//
// NOT CONCURRENCY SAFE
type OrthogonalStoreStateMachine[P any] interface {

	// Build returns:
	//   - database updates necessary for persisting the updated protocol sub-state and its *dependencies*.
	//     It may contain updates for the sub-state itself and for any dependency that is affected by the update.
	//     Deferred updates must be applied in a transaction to ensure atomicity.
	//
	// No errors are expected during normal operations.
	Build() (*transaction.DeferredBlockPersist, error)

	// EvolveState applies the state change(s) on sub-state P for the candidate block (under construction).
	// Information that potentially changes the Epoch state (compared to the parent block's state):
	//   - Service Events sealed in the candidate block
	//   - the candidate block's view (already provided at construction time)
	//
	// SAFETY REQUIREMENTS:
	//   - The seals for the execution results, from which the `sealedServiceEvents` originate,
	//     must be protocol compliant.
	//   - `sealedServiceEvents` must list the service Events in chronological order. This can be
	//     achieved by arranging the sealed execution results in order of increasing block height.
	//     Within each execution result, the service events are in chronological order.
	//   - EvolveState MUST be called for all candidate blocks, even if `sealedServiceEvents` is empty!
	//     This is because reaching a specific view can also trigger in state changes. (e.g. not having
	//     received the EpochCommit event for the next epoch, but approaching the end of the current epoch.)
	//
	// CAUTION:
	// Per convention, the input seals from the block payload have already been confirmed to be protocol compliant.
	// Hence, the service events in the sealed execution results represent the *honest* execution path. Therefore,
	// the sealed service events should encode a valid evolution of the protocol state -- provided the system smart
	// contracts are correct. As we can rule out byzantine attacks as the source of failures, the only remaining
	// sources of problems can be (a) bugs in the system smart contracts or (b) bugs in the node implementation.
	//   - A service event not representing a valid state transition despite all consistency checks passing is
	//     indicative of case (a) and _should be handled_ internally by the respective state machine. Otherwise,
	//     any bug or unforeseen edge cases in the system smart contracts would in consensus halt, due to errors
	//     while evolving the protocol state.
	//   - Consistency or sanity checks failing within the OrthogonalStoreStateMachine is likely the symptom of an
	//     internal bug in the node software or state corruption, i.e. case (b). This is the only scenario where the
	//     error return of this function is not nil. If such an exception is returned, continuing is not an option.
	//
	// No errors are expected during normal operations.
	EvolveState(sealedServiceEvents []flow.ServiceEvent) error

	// View returns the view associated with this state machine.
	// The view of the state machine equals the view of the block carrying the respective updates.
	View() uint64

	// ParentState returns parent state associated with this state machine.
	ParentState() P
}

// KeyValueStoreStateMachine is a type alias for a state machine that operates on an instance of KVStoreReader.
// StateMutator uses this type to store and perform operations on orthogonal state machines.
type KeyValueStoreStateMachine = OrthogonalStoreStateMachine[protocol.KVStoreReader]

// KeyValueStoreStateMachineFactory is an abstract factory interface for creating KeyValueStoreStateMachine instances.
// It is used separate creation of state machines from their usage, which allows less coupling and superior testability.
// For each concrete type injected in State Mutator a dedicated abstract factory has to be created.
type KeyValueStoreStateMachineFactory interface {
	// Create creates a new instance of an underlying type that operates on KV Store and is created for a specific candidate block.
	// No errors are expected during normal operations.
	Create(candidateView uint64, parentID flow.Identifier, parentState protocol.KVStoreReader, mutator KVStoreMutator) (KeyValueStoreStateMachine, error)
}
