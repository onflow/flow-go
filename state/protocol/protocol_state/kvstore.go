package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// This file contains versioned read and read-write interfaces to the Protocol State's
// key-value store and are used by the Protocol State Machine.
//
// When a key is added or removed, this requires a new protocol state version:
//  - Create a new versioned model in ./kvstore/models.go (eg. modelv3 if latest model is modelv2)
//  - Update the KVStoreReader and KVStoreAPI interfaces to include any new keys

// KVStoreReader is the latest read-only interface to the Protocol State key-value store
// at a particular block.
//
// Caution:
// Engineers evolving this interface must ensure that it is backwards-compatible
// with all versions of Protocol State Snapshots that can be retrieved from the local
// database, which should exactly correspond to the versioned model types defined in
// ./kvstore/models.go
type KVStoreReader interface {
	// ID returns an identifier for this key-value store snapshot by hashing internal fields.
	ID() flow.Identifier

	// v0

	VersionedEncodable

	// GetProtocolStateVersion returns the Protocol State Version that created the specific
	// Snapshot backing this interface instance. Slightly simplified, the Protocol State
	// Version defines the key-value store's data model (specifically, the set of all keys
	// and the respective type for each corresponding value).
	// Generally, changes in the protocol state version correspond to changes in the set
	// of key-value pairs which are supported, and which model is used for serialization.
	// The protocol state version is updated by UpdateKVStoreVersion service events.
	GetProtocolStateVersion() uint64

	// GetVersionUpgrade returns the upgrade version of protocol.
	// VersionUpgrade is a view-based activator that specifies the version which has to be applied
	// and the view from which on it has to be applied. It may return the current protocol version
	// with a past view if the upgrade has already been activated.
	GetVersionUpgrade() *ViewBasedActivator[uint64]

	// GetEpochStateID returns the state ID of the epoch state.
	// This is part of the most basic model and is used to commit the epoch state to the KV store.
	GetEpochStateID() flow.Identifier

	// v1

	// GetInvalidEpochTransitionAttempted returns a flag indicating whether epoch
	// fallback mode has been tentatively triggered on this fork.
	// Errors:
	//  - ErrKeyNotSupported if the respective entry does not exist in the
	//    Protocol State Snapshot that is backing the `Reader` interface.
	GetInvalidEpochTransitionAttempted() (bool, error)
}

// KVStoreAPI is the latest interface to the Protocol State key-value store which implements 'Prototype'
// pattern for replicating protocol state between versions.
//
// Caution:
// Engineers evolving this interface must ensure that it is backwards-compatible
// with all versions of Protocol State Snapshots that can be retrieved from the local
// database, which should exactly correspond to the versioned model types defined in
// ./kvstore/models.go
type KVStoreAPI interface {
	KVStoreReader

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
	//  - ErrIncompatibleVersionChange if replicating the Parent Snapshot into a Snapshot
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
	KVStoreReader

	// v0

	// SetVersionUpgrade sets the protocol upgrade version. This method is used
	// to update the Protocol State version when a flow.ProtocolStateVersionUpgrade is processed.
	// It contains the new version and the view at which it has to be applied.
	SetVersionUpgrade(version *ViewBasedActivator[uint64])

	// SetEpochStateID sets the state ID of the epoch state.
	// This method is used to commit the epoch state to the KV store when the state of the epoch is updated.
	SetEpochStateID(stateID flow.Identifier)

	// v1

	// SetInvalidEpochTransitionAttempted sets the epoch fallback mode flag.
	// Errors:
	//  - ErrKeyNotSupported if the key does not exist in the current model version.
	SetInvalidEpochTransitionAttempted(attempted bool) error
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
type OrthogonalStoreStateMachine[P any] interface {

	// Build returns:
	//   - database updates necessary for persisting the updated protocol sub-state and its *dependencies*.
	//     It may contain updates for the sub-state itself and for any dependency that is affected by the update.
	//     Deferred updates must be applied in a transaction to ensure atomicity.
	Build() protocol.DeferredBlockPersistOps

	// EvolveState applies the state change(s) on sub-state P for the candidate block (under construction).
	// Information that potentially changes the Epoch state (compared to the parent block's state):
	//   - Service Events sealed in the candidate block
	//   - the candidate block's view (already provided at construction time)
	//
	// CAUTION: EvolveState MUST be called for all candidate blocks, even if `sealedServiceEvents` is empty!
	// This is because also the absence of expected service events by a certain view can also result in the
	// Epoch state changing. (For example, not having received the EpochCommit event for the next epoch, but
	// approaching the end of the current epoch.)
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
type KeyValueStoreStateMachine = OrthogonalStoreStateMachine[KVStoreReader]

// KeyValueStoreStateMachineFactory is an abstract factory interface for creating KeyValueStoreStateMachine instances.
// It is used separate creation of state machines from their usage, which allows less coupling and superior testability.
// For each concrete type injected in State Mutator a dedicated abstract factory has to be created.
type KeyValueStoreStateMachineFactory interface {
	// Create creates a new instance of an underlying type that operates on KV Store and is created for a specific candidate block.
	// No errors are expected during normal operations.
	Create(candidateView uint64, parentID flow.Identifier, parentState KVStoreReader, mutator KVStoreMutator) (KeyValueStoreStateMachine, error)
}
