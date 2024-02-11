package kvstore

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
type modelv0 struct{}

var _ Reader = new(modelv0)
var _ API = new(modelv0)

func (model *modelv0) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(0, model)
}

func (model *modelv0) GetProtocolStateVersion() (uint64, error) {
	return 0, nil
}

func (model *modelv0) GetInvalidEpochTransitionAttempted() (bool, error) {
	return false, ErrKeyNotSupported
}

func (model *modelv0) SetInvalidEpochTransitionAttempted(_ bool) error {
	return ErrKeyNotSupported
}

// modelv1 is v1 of the Protocol State key-value store.
// This represents the first model version which will be considered "latest" by any
// deployed software version.
type modelv1 struct {
	// InvalidEpochTransitionAttempted encodes whether an invalid epoch transition
	// has been detected in this fork. Under normal operations, this value is false.
	// Node-internally, the EpochFallback notification is emitted when a block is
	// finalized that changes this flag from false to true.
	//
	// Currently, the only possible state transition is false → true.
	InvalidEpochTransitionAttempted bool
}

var _ Reader = new(modelv1)
var _ API = new(modelv1)

func (model *modelv1) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(1, model)
}

func (model *modelv1) GetProtocolStateVersion() (uint64, error) {
	return 1, nil
}

func (model *modelv1) GetInvalidEpochTransitionAttempted() (bool, error) {
	return model.InvalidEpochTransitionAttempted, nil
}

func (model *modelv1) SetInvalidEpochTransitionAttempted(attempted bool) error {
	model.InvalidEpochTransitionAttempted = attempted
	return nil
}
