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
// Service Events sealed in one block). In other words, each state machine S0, S1,… has full visibility
// into the inputs, but each draws their on independent conclusions (maintain their own exclusive state).
// There is no information exchange between the state machines; one state machines cannot read the state
// of another.
// We emphasize that this architecture choice does not prevent us of from implementing sequential state
// machines for certain use-cases. For example: state machine A provides its output as input to another
// state machine B. Here, the order of running the state machines matter. This order-dependency is not
// supported by the Protocol State, which executed the state machines in an arbitrary order. Therefore,
// if we need state machines to be executed in some order, we have bundle them into one composite state
// machine (conceptually a processing pipeline) by hand. The composite state machine's execution as a
// whole can then be managed by the Protocol State, because the composite state machine is orthogonal
// to all other remaining state machines.
// Requiring all State Machines to be orthogonal is a deliberate design choice. Thereby the default is
// favouring modularity and strong logical independence. This is very beneficial for managing complexity
// in the long term.
//
// Key-Value-Store:
// The Flow protocol defines the Key-Value-Store's state 𝒫 as the composition of disjoint sub-states
// P0, P1, …, Pj. Formally, we write 𝒫 = P0 ⊗ P1 ⊗ … ⊗ Pj, where '⊗' denotes the product state. We
// loosely associate each P0, P1,… with one specific key-value entry in the store. Correspondingly,
// we have conceptually independent state machines S0, S1,… operating each on their own respective
// sub-state P0, P1,… . A one-to-one correspondence between kv-entry and state machine should be the
// default, but is not strictly required. However, the strong requirement is that no entry is operated
// on my more than one state machine.
//
// Formally we write:
//   - The overall protocol state 𝒫 is composed of disjoint substates 𝒫 = P0 ⊗ P1 ⊗  ... ⊗ Pj
//   - For each state Pi, we have a dedicated state machine Si that exclusively operates on Pi
//   - The state machines can be formalized as orthogonal regions of the composite state machine
//     𝒮 = S0 ⊗ S1 ⊗  ... ⊗ Sj. (Technically, we represent the state machine by its state-transition
//     function. All other details of the state machine are implicit.)
//   - The state machine 𝒮 being in state 𝒫 and observing the input ξ = x0·x1·x2·..·xz will output
//     state 𝒫'. To emphasize that a certain state machine 𝒮 exclusively operates on state 𝒫, we write
//     𝒮[𝒫] = S0[P0] ⊗ S1[P1] ⊗ ... ⊗ Sj[Pj]
//     Observing the events ξ the output state 𝒫' is
//     𝒫' = 𝒮[𝒫](ξ) = S0[P0](ξ) ⊗ S1[P1](ξ) ⊗ ... ⊗ Sj[Pj](ξ) = P'0 ⊗ P'1 ⊗  ... ⊗ P'j
//     Where each state machine Si individually generated the output state Si[Pi](ξ) = P'i
//
// Input ξ:
// Conceptually, the consensus leader first executes these state machines during their block building
// process. At this point, the ID of the final block is unknown. Nevertheless, some part of the payload
// construction already happened, because at the sealed execution results are used as an input below.
// There is a large degree of freedom what part of the block we permit as possible inputs to the state
// machines. At the moment, the primary purpose is for the execution environment (with results undergone
// verification and sealing) to send Service Events to the protocol layer. Therefore, the current
// convention is:
//  1. At time of state machine construction (for each block), the Protocol State framework provides:
//     • candidateView: view of the block currently under construction
//     • parentID: parent block's ID (generally used by state machines to read their respective sub-state)
//  2. The Service Events sealed in the candidate block (under construction)
//     are given to each state machine via the `EvolveState(..)` call.
//     CAUTION: `EvolveState(..)` MUST be called for all candidate blocks, even if there are no seals
//     (or an empty payload).
//
// The Protocol State is the framework, which orchestrates the orthogonal state machines,
// feeds them with inputs, post-processes the outputs and overall manages state machines' life-cycle
// from block to block. New key-value pairs and corresponding state machines can easily be added
// by implementing the following interface (state machine) and adding a new entry to the KV store.
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
