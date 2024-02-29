package kvstore

import "github.com/onflow/flow-go/state/protocol/protocol_state"

type upgradableModel struct {
	VersionUpgrade protocol_state.ViewBasedActivator[uint64]
}

func (model *upgradableModel) SetProtocolStateVersion(activator protocol_state.ViewBasedActivator[uint64]) {
	model.VersionUpgrade = activator
}

// This file contains the concrete types that define the structure of the
// underlying key-value store for a particular Protocol State version.
// Essentially enumerating the set of keys and values that are supported.
//
// When a key is added or removed, this requires a new protocol state version:
//  - Create a new versioned model in models.go (eg. modelv3 if latest model is modelv2)
//  - Update the Reader and API interfaces to include any new keys

// modelv0 is v0 of the Protocol State key-value store.
// This model version is not intended to ever be the latest version supported by
// any software version. Since it is important that the store support managing
// different model version, this is here so that we can test the implementation
// with multiple supported KV model versions from the beginning.
type modelv0 struct {
	upgradableModel
}

var _ protocol_state.Reader = new(modelv0)
var _ protocol_state.API = new(modelv0)

// VersionedEncode encodes the key-value store, returning the version separately
// from the encoded bytes.
// No errors are expected during normal operation.
func (model *modelv0) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(model.GetProtocolStateVersion(), model)
}

// GetProtocolStateVersion returns the Protocol State version.
func (model *modelv0) GetProtocolStateVersion() uint64 {
	return 0
}

// GetInvalidEpochTransitionAttempted returns ErrKeyNotSupported.
func (model *modelv0) GetInvalidEpochTransitionAttempted() (bool, error) {
	return false, ErrKeyNotSupported
}

// SetInvalidEpochTransitionAttempted returns ErrKeyNotSupported.
func (model *modelv0) SetInvalidEpochTransitionAttempted(_ bool) error {
	return ErrKeyNotSupported
}

// modelv1 is v1 of the Protocol State key-value store.
// This represents the first model version which will be considered "latest" by any
// deployed software version.
type modelv1 struct {
	upgradableModel

	// InvalidEpochTransitionAttempted encodes whether an invalid epoch transition
	// has been detected in this fork. Under normal operations, this value is false.
	// Node-internally, the EpochFallback notification is emitted when a block is
	// finalized that changes this flag from false to true.
	//
	// Currently, the only possible state transition is false → true.
	InvalidEpochTransitionAttempted bool
}

var _ protocol_state.Reader = new(modelv1)
var _ protocol_state.API = new(modelv1)

// VersionedEncode encodes the key-value store, returning the version separately
// from the encoded bytes.
// No errors are expected during normal operation.
func (model *modelv1) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(model.GetProtocolStateVersion(), model)
}

// GetProtocolStateVersion returns the Protocol State version.
func (model *modelv1) GetProtocolStateVersion() uint64 {
	return 1
}

// GetInvalidEpochTransitionAttempted returns the InvalidEpochTransitionAttempted flag.
// This key must have a value set and will never return ErrKeyNotSet.
// No errors are expected during normal operation.
func (model *modelv1) GetInvalidEpochTransitionAttempted() (bool, error) {
	return model.InvalidEpochTransitionAttempted, nil
}

// SetInvalidEpochTransitionAttempted sets the InvalidEpochTransitionAttempted flag.
// No errors are expected during normal operation.
func (model *modelv1) SetInvalidEpochTransitionAttempted(attempted bool) error {
	model.InvalidEpochTransitionAttempted = attempted
	return nil
}
