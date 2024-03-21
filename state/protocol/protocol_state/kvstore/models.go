package kvstore

import (
	"fmt"

	"github.com/huandu/go-clone/generic" //nolint:goimports

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

type upgradableModel struct {
	VersionUpgrade *protocol_state.ViewBasedActivator[uint64]
}

// SetVersionUpgrade sets the protocol upgrade version. This method is used
// to update the Protocol State version when a flow.ProtocolStateVersionUpgrade is processed.
// It contains the new version and the view at which it has to be applied.
func (model *upgradableModel) SetVersionUpgrade(activator *protocol_state.ViewBasedActivator[uint64]) {
	model.VersionUpgrade = activator
}

// GetVersionUpgrade returns the upgrade version of protocol.
// VersionUpgrade is a view-based activator that specifies the version which has to be applied
// and the view from which on it has to be applied. It may return the current protocol version
// with a past view if the upgrade has already been activated.
func (model *upgradableModel) GetVersionUpgrade() *protocol_state.ViewBasedActivator[uint64] {
	return model.VersionUpgrade
}

// This file contains the concrete types that define the structure of the
// underlying key-value store for a particular Protocol State version.
// Essentially enumerating the set of keys and values that are supported.
//
// When a key is added or removed, this requires a new protocol state version:
//  - Create a new versioned model in models.go (eg. modelv3 if latest model is modelv2)
//  - Update the KVStoreReader and KVStoreAPI interfaces to include any new keys

// modelv0 is v0 of the Protocol State key-value store.
// This model version is not intended to ever be the latest version supported by
// any software version. Since it is important that the store support managing
// different model version, this is here so that we can test the implementation
// with multiple supported KV model versions from the beginning.
type modelv0 struct {
	upgradableModel
	EpochStateID flow.Identifier
}

var _ protocol_state.KVStoreAPI = (*modelv0)(nil)
var _ protocol_state.KVStoreMutator = (*modelv0)(nil)

// ID returns an identifier for this key-value store snapshot by hashing internal fields.
func (model *modelv0) ID() flow.Identifier { return flow.MakeID(model) }

// Replicate instantiates a Protocol State Snapshot of the given `protocolVersion`.
// It clones existing snapshot if `protocolVersion = 0` and performs a migration if `protocolVersion = 1`.
// Expected errors during normal operations:
//   - ErrIncompatibleVersionChange if replicating the Parent Snapshot into a Snapshot
//     with the specified `protocolVersion` is not supported.
func (model *modelv0) Replicate(protocolVersion uint64) (protocol_state.KVStoreMutator, error) {
	version := model.GetProtocolStateVersion()
	if version == protocolVersion {
		// no need for migration, return a complete copy
		return clone.Clone(model), nil
	} else if protocolVersion != 1 {
		return nil, fmt.Errorf("unsupported replication version %d, expect %d: %w",
			protocolVersion, 1, ErrIncompatibleVersionChange)
	}

	// perform actual replication to the next version
	v1 := &modelv1{
		modelv0:                         clone.Clone(*model),
		InvalidEpochTransitionAttempted: false,
	}
	return v1, nil
}

// VersionedEncode encodes the key-value store, returning the version separately
// from the encoded bytes.
// No errors are expected during normal operation.
func (model *modelv0) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(model.GetProtocolStateVersion(), model)
}

// GetProtocolStateVersion returns the version of the Protocol State Snapshot
// that is backing the `Reader` interface. It is the protocol version that originally
// created the Protocol State Snapshot. Changes in the protocol state version
// correspond to changes in the set of key-value pairs which are supported,
// and which model is used for serialization.
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

func (model *modelv0) GetEpochStateID() flow.Identifier {
	return model.EpochStateID
}

func (model *modelv0) SetEpochStateID(id flow.Identifier) {
	model.EpochStateID = id
}

// modelv1 is v1 of the Protocol State key-value store.
// This represents the first model version which will be considered "latest" by any
// deployed software version.
type modelv1 struct {
	modelv0

	// InvalidEpochTransitionAttempted encodes whether an invalid epoch transition
	// has been detected in this fork. Under normal operations, this value is false.
	// Node-internally, the EpochFallback notification is emitted when a block is
	// finalized that changes this flag from false to true.
	//
	// Currently, the only possible state transition is false → true.
	InvalidEpochTransitionAttempted bool
}

var _ protocol_state.KVStoreAPI = (*modelv1)(nil)
var _ protocol_state.KVStoreMutator = (*modelv1)(nil)

// ID returns an identifier for this key-value store snapshot by hashing internal fields.
func (model *modelv1) ID() flow.Identifier { return flow.MakeID(model) }

// Replicate instantiates a Protocol State Snapshot of the given `protocolVersion`.
// It clones existing snapshot if `protocolVersion = 1`, other versions are not supported yet.
// Expected errors during normal operations:
//   - ErrIncompatibleVersionChange if replicating the Parent Snapshot into a Snapshot
//     with the specified `protocolVersion` is not supported.
func (model *modelv1) Replicate(protocolVersion uint64) (protocol_state.KVStoreMutator, error) {
	version := model.GetProtocolStateVersion()
	if version == protocolVersion {
		// no need for migration, return a complete copy
		return clone.Clone(model), nil
	} else {
		return nil, fmt.Errorf("unsupported replication version %d: %w",
			protocolVersion, ErrIncompatibleVersionChange)
	}
}

// VersionedEncode encodes the key-value store, returning the version separately
// from the encoded bytes.
// No errors are expected during normal operation.
func (model *modelv1) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(model.GetProtocolStateVersion(), model)
}

// GetProtocolStateVersion returns the version of the Protocol State Snapshot
// that is backing the `Reader` interface. It is the protocol version that originally
// created the Protocol State Snapshot. Changes in the protocol state version
// correspond to changes in the set of key-value pairs which are supported,
// and which model is used for serialization.
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

// TODO: this is temporary, only for testing bootstrapping
func NewLatestKVStore(epochStateID flow.Identifier) protocol_state.KVStoreAPI {
	return &modelv1{
		modelv0: modelv0{
			upgradableModel: upgradableModel{},
			EpochStateID:    epochStateID,
		},
		InvalidEpochTransitionAttempted: false,
	}
}
