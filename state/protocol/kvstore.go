package protocol

import "github.com/onflow/flow-go/model/flow"

// VersionedEncodable defines the interface for a versioned key-value store independent
// of the set of keys which are supported. This allows the storage layer to support
// storing different key-value model versions within the same software version.
type VersionedEncodable interface {
	// VersionedEncode encodes the key-value store, returning the version separately
	// from the encoded bytes.
	// No errors are expected during normal operation.
	VersionedEncode() (uint64, []byte, error)
}

// ViewBasedActivator allows setting value that will be active from specific view.
type ViewBasedActivator[T any] struct {
	Data           T
	ActivationView uint64 // first view at which Data will take effect
}

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
