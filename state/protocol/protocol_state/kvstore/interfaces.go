package kvstore

// This file contains versioned read and read-write interfaces to the Protocol State
// key-value store and are used by the Protocol State Machine.
//
// When a key is added or removed, this requires a new protocol state version:
//  - Create a new versioned model in models.go (eg. modelv3 if latest model is modelv2)
//  - Update the Reader and API interfaces to include any new keys

// Reader is the latest read-only interface to the Protocol State key-value store
// at a particular block.
// It is backward-compatible with all versioned model types defined in the models.go
// for this software version.
type Reader interface {
	// v0

	VersionedEncodable

	// GetProtocolStateVersion returns the current Protocol State version.
	// The Protocol State version specifies the current version of the Protocol State,
	// including the key-value store. Changes in the protocol state version
	// correspond to changes in the set of key-value pairs which are supported,
	// and which model is used for serialization.
	// It can be updated by an UpdateKVStoreVersion service event.
	// No errors are expected under normal operation.
	GetProtocolStateVersion() (uint64, error)

	// v1

	// GetInvalidEpochTransitionAttempted returns a flag indicating whether epoch
	// fallback mode has been tentatively triggered on this fork.
	// Errors:
	//  - ErrKeyNotSupported if the key does not exist in the current model version.
	GetInvalidEpochTransitionAttempted() (bool, error)
}

// API is the latest read-writer interface to the Protocol State key-value store.
// It is backward-compatible with all versioned model types defined in the models.go
// for this software version.
type API interface {
	Reader

	// v1

	// SetInvalidEpochTransitionAttempted sets the epoch fallback mode flag.
	// Errors:
	//  - ErrKeyNotSupported if the key does not exist in the current model version.
	SetInvalidEpochTransitionAttempted(attempted bool) error
}
