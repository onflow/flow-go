package protocol_state

import "github.com/onflow/flow-go/model/flow"

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
	// Since we are potentially converting from the older protocol version of the parent to
	// a newer version, it is possible that the replicated state snapshot is incomplete
	// or otherwise requires further inputs or mutations to result in the valid Protocol
	// State Snapshot according to the new protocol version. Therefore, we represent the outcome
	// of the replication as a `KVStoreMutator`
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

	// v1

	// SetInvalidEpochTransitionAttempted sets the epoch fallback mode flag.
	// Errors:
	//  - ErrKeyNotSupported if the key does not exist in the current model version.
	SetInvalidEpochTransitionAttempted(attempted bool) error
}

// KeyValueStoreStateMachine implements a low-level interface for state-changing operations on the key-value store.
// It is used by higher level logic to evolve the protocol state when certain events that are stored in blocks are observed.
// The KeyValueStoreStateMachine is stateful and internally tracks the current state of key-value store.
// A separate instance is created for each block that is being processed.
type KeyValueStoreStateMachine interface {
	// Build returns updated key-value store model, state ID and a flag indicating if there were any changes.
	Build() (updatedState KVStoreReader, stateID flow.Identifier, hasChanges bool)

	// ProcessUpdate updates the current state of key-value store.
	// KeyValueStoreStateMachine captures only a subset of all service events, those that are relevant for the KV store. All other events are ignored.
	// Implementors MUST ensure KeyValueStoreStateMachine is left in functional state if an invalid service event has been supplied.
	// Expected errors indicating that we have observed an invalid service event from protocol's point of view.
	// 	- `protocol.InvalidServiceEventError` - if the service event is invalid for the current protocol state.
	ProcessUpdate(update *flow.ServiceEvent) error

	// View returns the view that is associated with this KeyValueStoreStateMachine.
	// The view of the KeyValueStoreStateMachine equals the view of the block carrying the respective updates.
	View() uint64

	// ParentState returns parent state that is associated with this state machine.
	ParentState() KVStoreReader
}
