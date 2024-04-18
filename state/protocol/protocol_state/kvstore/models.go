package kvstore

import (
	"fmt"

	"github.com/huandu/go-clone/generic" //nolint:goimports

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

// This file contains the concrete types that define the structure of the underlying key-value store
// for a particular Protocol State version.
// Essentially enumerating the set of keys and values that are supported.
// When a key is added or removed, this requires a new protocol state version.
// To use new version of the protocol state, create a new versioned model in models.go (eg. modelv3 if latest model is modelv2)
// ATTENTION: All models should be public with public fields otherwise the encoding/decoding will not work.

// UpgradableModel is a utility struct that must be embedded in all model versions to provide
// a common interface for managing protocol version upgrades.
type UpgradableModel struct {
	VersionUpgrade *protocol.ViewBasedActivator[uint64]
}

// SetVersionUpgrade sets the protocol upgrade version. This method is used
// to update the Protocol State version when a flow.ProtocolStateVersionUpgrade is processed.
// It contains the new version and the view at which it has to be applied.
func (model *UpgradableModel) SetVersionUpgrade(activator *protocol.ViewBasedActivator[uint64]) {
	model.VersionUpgrade = activator
}

// GetVersionUpgrade returns the upgrade version of protocol.
// VersionUpgrade is a view-based activator that specifies the version which has to be applied
// and the view from which it has to be applied. It may return the current protocol version
// with a past view if the upgrade has already been activated.
func (model *UpgradableModel) GetVersionUpgrade() *protocol.ViewBasedActivator[uint64] {
	return model.VersionUpgrade
}

// This file contains the concrete types that define the structure of the
// underlying key-value store for a particular Protocol State version.
// Essentially enumerating the set of keys and values that are supported.
//
// When a key is added or removed, this requires a new protocol state version:
//  - Create a new versioned model in models.go (eg. modelv3 if latest model is modelv2)
//  - Update the KVStoreReader and KVStoreAPI interfaces to include any new keys

// Modelv0 is v0 of the Protocol State key-value store.
// This model version is not intended to ever be the latest version supported by
// any software version. Since it is important that the store support managing
// different model version, this is here so that we can test the implementation
// with multiple supported KV model versions from the beginning.
type Modelv0 struct {
	UpgradableModel
	EpochStateID flow.Identifier
}

var _ protocol_state.KVStoreAPI = (*Modelv0)(nil)
var _ protocol_state.KVStoreMutator = (*Modelv0)(nil)

// ID returns an identifier for this key-value store snapshot by hashing internal fields.
func (model *Modelv0) ID() flow.Identifier { return flow.MakeID(model) }

// Replicate instantiates a Protocol State Snapshot of the given `protocolVersion`.
// It clones existing snapshot and performs a migration if `protocolVersion = 1`.
// Expected errors during normal operations:
//   - ErrIncompatibleVersionChange if replicating the Parent Snapshot into a Snapshot
//     with the specified `protocolVersion` is not supported.
func (model *Modelv0) Replicate(protocolVersion uint64) (protocol_state.KVStoreMutator, error) {
	version := model.GetProtocolStateVersion()
	if version == protocolVersion {
		// no need for migration, return a complete copy
		return clone.Clone(model), nil
	} else if protocolVersion != 1 {
		return nil, fmt.Errorf("unsupported replication version %d, expect %d: %w",
			protocolVersion, 1, ErrIncompatibleVersionChange)
	}

	// perform actual replication to the next version
	v1 := &Modelv1{
		Modelv0: clone.Clone(*model),
	}
	return v1, nil
}

// VersionedEncode encodes the key-value store, returning the version separately
// from the encoded bytes.
// No errors are expected during normal operation.
func (model *Modelv0) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(model.GetProtocolStateVersion(), model)
}

// GetProtocolStateVersion returns the version of the Protocol State Snapshot
// that is backing the `Reader` interface. It is the protocol version that originally
// created the Protocol State Snapshot. Changes in the protocol state version
// correspond to changes in the set of key-value pairs which are supported,
// and which model is used for serialization.
func (model *Modelv0) GetProtocolStateVersion() uint64 {
	return 0
}

// GetEpochStateID returns the state ID of the epoch state.
// This is part of the most basic model and is used to commit the epoch state to the KV store.
func (model *Modelv0) GetEpochStateID() flow.Identifier {
	return model.EpochStateID
}

// SetEpochStateID sets the state ID of the epoch state.
// This method is used to commit the epoch state to the KV store when the state of the epoch is updated.
func (model *Modelv0) SetEpochStateID(id flow.Identifier) {
	model.EpochStateID = id
}

// Modelv1 is v1 of the Protocol State key-value store.
// This represents the first model version which will be considered "latest" by any
// deployed software version.
type Modelv1 struct {
	Modelv0
}

var _ protocol_state.KVStoreAPI = (*Modelv1)(nil)
var _ protocol_state.KVStoreMutator = (*Modelv1)(nil)

// ID returns an identifier for this key-value store snapshot by hashing internal fields.
func (model *Modelv1) ID() flow.Identifier { return flow.MakeID(model) }

// Replicate instantiates a Protocol State Snapshot of the given `protocolVersion`.
// It clones existing snapshot if `protocolVersion = 1`, other versions are not supported yet.
// Expected errors during normal operations:
//   - ErrIncompatibleVersionChange if replicating the Parent Snapshot into a Snapshot
//     with the specified `protocolVersion` is not supported.
func (model *Modelv1) Replicate(protocolVersion uint64) (protocol_state.KVStoreMutator, error) {
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
func (model *Modelv1) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(model.GetProtocolStateVersion(), model)
}

// GetProtocolStateVersion returns the version of the Protocol State Snapshot
// that is backing the `Reader` interface. It is the protocol version that originally
// created the Protocol State Snapshot. Changes in the protocol state version
// correspond to changes in the set of key-value pairs which are supported,
// and which model is used for serialization.
func (model *Modelv1) GetProtocolStateVersion() uint64 {
	return 1
}

// NewDefaultKVStore constructs a default Key-Value Store of the *latest* protocol version for bootstrapping.
// Currently, the KV store is largely empty.
// TODO: Shortcut in bootstrapping; we will probably have to start with a non-empty KV store in the future;
// Potentially we may need to carry over the KVStore during a spork (with possible migrations).
func NewDefaultKVStore(epochStateID flow.Identifier) protocol_state.KVStoreAPI {
	return &Modelv1{
		Modelv0: Modelv0{
			UpgradableModel: UpgradableModel{},
			EpochStateID:    epochStateID,
		},
	}
}

// NewKVStoreV0 constructs a KVStore using the v0 model. This is used to test
// version upgrades, from v0 to v1.
func NewKVStoreV0(epochStateID flow.Identifier) protocol_state.KVStoreAPI {
	return &Modelv0{
		UpgradableModel: UpgradableModel{},
		EpochStateID:    epochStateID,
	}
}
