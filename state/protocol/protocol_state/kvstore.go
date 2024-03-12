package protocol_state

import "github.com/onflow/flow-go/model/flow"

// ViewBasedActivator allows setting value that will be active from specific view.
type ViewBasedActivator[T any] struct {
	Data           T
	ActivationView uint64 // first view at which Data will take effect
}

// VersionedEncodable defines the interface for a versioned key-value store independent
// of the set of keys which are supported. This allows the storage layer to support
// storing different key-value model versions within the same software version.
type VersionedEncodable interface {
	// VersionedEncode encodes the key-value store, returning the version separately
	// from the encoded bytes.
	// No errors are expected during normal operation.
	VersionedEncode() (uint64, []byte, error)
}

// This file contains versioned read and read-write interfaces to the Protocol State's
// key-value store and are used by the Protocol State Machine.
//
// When a key is added or removed, this requires a new protocol state version:
//  - Create a new versioned model in ./kvstore/models.go (eg. modelv3 if latest model is modelv2)
//  - Update the KVStoreReader and KVStoreAPI interfaces to include any new keys

// KVStoreReader is the latest read-only interface to the Protocol State key-value store
// at a particular block.
// It is backward-compatible with all versioned model types defined in the models.go
// for this software version.
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
	// and the view at which it has to be applied. It may contain an old value if the upgrade has already applied.
	// It can be updated by an flow.ProtocolStateVersionUpgrade service event.
	GetVersionUpgrade() *ViewBasedActivator[uint64]

	// v1

	// GetInvalidEpochTransitionAttempted returns a flag indicating whether epoch
	// fallback mode has been tentatively triggered on this fork.
	// Errors:
	//  - ErrKeyNotSupported if the key does not exist in the current model version.
	GetInvalidEpochTransitionAttempted() (bool, error)
}

// KVStoreAPI is the latest read-writer interface to the Protocol State key-value store.
// It is backward-compatible with all versioned model types defined in the models.go
// for this software version.
type KVStoreAPI interface {
	KVStoreReader

	// Clone returns a deep copy of the KVStoreAPI.
	// This is used to create a new instance of the KVStoreAPI to avoid mutating the original.
	Clone() KVStoreAPI

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
