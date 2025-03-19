package kvstore

import (
	"fmt"

	clone "github.com/huandu/go-clone/generic" //nolint:goimports

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
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
// and the view from which on it has to be applied. After an upgrade activation view has passed,
// the (version, view) data remains in the state until the next upgrade is scheduled (essentially
// persisting the most recent past update until a subsequent update is scheduled).
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
	EpochStateID                flow.Identifier
	EpochExtensionViewCount     uint64
	FinalizationSafetyThreshold uint64
}

var _ protocol_state.KVStoreAPI = (*Modelv0)(nil)
var _ protocol_state.KVStoreMutator = (*Modelv0)(nil)

// ID returns an identifier for this key-value store snapshot by hashing internal fields and version number.
func (model *Modelv0) ID() flow.Identifier {
	return makeVersionedModelID(model)
}

// Replicate instantiates a Protocol State Snapshot of the given protocolVersion.
// It clones existing snapshot if protocolVersion = currentVersion.
// It transitions to next version if protocolVersion = currentVersion+1.
// Expected errors during normal operations:
//   - ErrIncompatibleVersionChange if replicating the Parent Snapshot into a Snapshot
//     with the specified `protocolVersion` is not supported.
func (model *Modelv0) Replicate(protocolVersion uint64) (protocol_state.KVStoreMutator, error) {
	currentVersion := model.GetProtocolStateVersion()
	if currentVersion == protocolVersion {
		// no need for migration, return a complete copy
		return clone.Clone(model), nil
	}
	nextVersion := currentVersion + 1
	if protocolVersion != nextVersion {
		return nil, fmt.Errorf("unsupported replication version %d, expect %d: %w",
			protocolVersion, nextVersion, ErrIncompatibleVersionChange)
	}

	// perform actual replication to the next version
	v1 := &Modelv1{
		Modelv0: clone.Clone(*model),
	}
	if v1.GetProtocolStateVersion() != protocolVersion {
		return nil, fmt.Errorf("sanity check: replicate resulted in unexpected version (%d != %d)", v1.GetProtocolStateVersion(), protocolVersion)
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

// SetEpochExtensionViewCount sets the number of views for a hypothetical epoch extension.
// Expected errors during normal operations:
//   - kvstore.ErrInvalidValue - if the view count is less than FinalizationSafetyThreshold*2.
func (model *Modelv0) SetEpochExtensionViewCount(viewCount uint64) error {
	// Strictly speaking it should be perfectly fine to use a value viewCount >= model.FinalizationSafetyThreshold.
	// By using a slightly higher value (factor of 2), we ensure that each extension spans a sufficiently big time
	// window for the human governance committee to submit a valid epoch recovery transaction.
	if viewCount < model.FinalizationSafetyThreshold*2 {
		return fmt.Errorf("invalid view count %d, expect at least %d: %w", viewCount, model.FinalizationSafetyThreshold*2, ErrInvalidValue)
	}
	model.EpochExtensionViewCount = viewCount
	return nil
}

// GetEpochExtensionViewCount returns the number of views for a hypothetical epoch extension. Note
// that this value can change at runtime (through a service event). When a new extension is added,
// the view count is used right at this point in the protocol state's evolution. In other words,
// different extensions can have different view counts.
func (model *Modelv0) GetEpochExtensionViewCount() uint64 {
	return model.EpochExtensionViewCount
}

func (model *Modelv0) GetFinalizationSafetyThreshold() uint64 {
	return model.FinalizationSafetyThreshold
}

// GetCadenceComponentVersion always returns ErrKeyNotSupported because this field is unsupported for Modelv0.
func (model *Modelv0) GetCadenceComponentVersion() (protocol.MagnitudeOfChangeVersion, error) {
	return protocol.MagnitudeOfChangeVersion{}, ErrKeyNotSupported
}

// GetCadenceComponentVersionUpgrade always returns nil because this field is unsupported for Modelv0.
func (model *Modelv0) GetCadenceComponentVersionUpgrade() *protocol.ViewBasedActivator[protocol.MagnitudeOfChangeVersion] {
	return nil
}

// GetExecutionComponentVersion always returns ErrKeyNotSupported because this field is unsupported for Modelv0.
func (model *Modelv0) GetExecutionComponentVersion() (protocol.MagnitudeOfChangeVersion, error) {
	return protocol.MagnitudeOfChangeVersion{}, ErrKeyNotSupported
}

// GetExecutionComponentVersionUpgrade always returns nil because this field is unsupported for Modelv0.
func (model *Modelv0) GetExecutionComponentVersionUpgrade() *protocol.ViewBasedActivator[protocol.MagnitudeOfChangeVersion] {
	return nil
}

// GetExecutionMeteringParameters always returns ErrKeyNotSupported because this field is unsupported for Modelv0.
func (model *Modelv0) GetExecutionMeteringParameters() (protocol.ExecutionMeteringParameters, error) {
	return protocol.ExecutionMeteringParameters{}, ErrKeyNotSupported
}

// GetExecutionMeteringParametersUpgrade always returns nil because this field is unsupported for Modelv0.
func (model *Modelv0) GetExecutionMeteringParametersUpgrade() *protocol.ViewBasedActivator[protocol.ExecutionMeteringParameters] {
	return nil
}

// Modelv1 is v1 of the Protocol State key-value store.
// This represents the first model version which will be considered "latest" by any
// deployed software version.
type Modelv1 struct {
	Modelv0
}

var _ protocol_state.KVStoreAPI = (*Modelv1)(nil)
var _ protocol_state.KVStoreMutator = (*Modelv1)(nil)

// ID returns an identifier for this key-value store snapshot by hashing internal fields and version number.
func (model *Modelv1) ID() flow.Identifier {
	return makeVersionedModelID(model)
}

// Replicate instantiates a Protocol State Snapshot of the given protocolVersion.
// It clones existing snapshot if protocolVersion = currentVersion.
// It transitions to next version if protocolVersion = currentVersion+1.
// Expected errors during normal operations:
//   - ErrIncompatibleVersionChange if replicating the Parent Snapshot into a Snapshot
//     with the specified `protocolVersion` is not supported.
func (model *Modelv1) Replicate(protocolVersion uint64) (protocol_state.KVStoreMutator, error) {
	currentVersion := model.GetProtocolStateVersion()
	if currentVersion == protocolVersion {
		// no need for migration, return a complete copy
		return clone.Clone(model), nil
	}
	nextVersion := currentVersion + 1
	if protocolVersion != nextVersion {
		// can only Replicate into model with numerically consecutive version
		return nil, fmt.Errorf("unsupported replication version %d, expect %d: %w",
			protocolVersion, nextVersion, ErrIncompatibleVersionChange)
	}

	// perform actual replication to the next version
	v2 := &Modelv2{
		Modelv1: clone.Clone(*model),
		// Execution component versions and metering parameters are explicitly undefined when upgrading to v2
		CadenceComponentVersion: protocol.UpdatableField[protocol.MagnitudeOfChangeVersion]{
			CurrentValue: protocol.UndefinedMagnitudeOfChangeVersion,
		},
		ExecutionComponentVersion: protocol.UpdatableField[protocol.MagnitudeOfChangeVersion]{
			CurrentValue: protocol.UndefinedMagnitudeOfChangeVersion,
		},
		ExecutionMeteringParameters: protocol.UpdatableField[protocol.ExecutionMeteringParameters]{
			CurrentValue: protocol.UndefinedExecutionMeteringParameters,
		},
	}
	if v2.GetProtocolStateVersion() != protocolVersion {
		return nil, fmt.Errorf("sanity check: replicate resulted in unexpected version (%d != %d)", v2.GetProtocolStateVersion(), protocolVersion)
	}
	return v2, nil
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

// Modelv2 adds fields for execution versioning and metering, and reflects a behavioural
// change for EFM Recovery.
// This version adds the following changes:
//   - Adds execution versioning and metering fields
//   - Non-system-chunk service event validation support (adds ChunkBody.ServiceEventCount field)
//   - EFM Recovery (adds EpochCommit.DKGIndexMap field)
type Modelv2 struct {
	Modelv1
	ExecutionMeteringParameters protocol.UpdatableField[protocol.ExecutionMeteringParameters]
	ExecutionComponentVersion   protocol.UpdatableField[protocol.MagnitudeOfChangeVersion]
	CadenceComponentVersion     protocol.UpdatableField[protocol.MagnitudeOfChangeVersion]
}

// ID returns an identifier for this key-value store snapshot by hashing internal fields and version number.
func (model *Modelv2) ID() flow.Identifier {
	return makeVersionedModelID(model)
}

// Replicate instantiates a Protocol State Snapshot of the given protocolVersion.
// It clones existing snapshot if protocolVersion = currentVersion, other versions are not supported yet.
// Expected errors during normal operations:
//   - ErrIncompatibleVersionChange if replicating the Parent Snapshot into a Snapshot
//     with the specified `protocolVersion` is not supported.
func (model *Modelv2) Replicate(protocolVersion uint64) (protocol_state.KVStoreMutator, error) {
	currentVersion := model.GetProtocolStateVersion()
	if currentVersion == protocolVersion {
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
func (model *Modelv2) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(model.GetProtocolStateVersion(), model)
}

// GetProtocolStateVersion returns the version of the Protocol State Snapshot
// that is backing the `Reader` interface. It is the protocol version that originally
// created the Protocol State Snapshot. Changes in the protocol state version
// correspond to changes in the set of key-value pairs which are supported,
// and which model is used for serialization.
func (model *Modelv2) GetProtocolStateVersion() uint64 {
	return 2
}

// GetCadenceComponentVersion returns the current Cadence component version from Modelv2.
// Returns kvstore.ErrKeyNotSet if the key has no value
func (model *Modelv2) GetCadenceComponentVersion() (protocol.MagnitudeOfChangeVersion, error) {
	if model.CadenceComponentVersion.CurrentValue == protocol.UndefinedMagnitudeOfChangeVersion {
		return protocol.UndefinedMagnitudeOfChangeVersion, ErrKeyNotSet
	}
	return model.CadenceComponentVersion.CurrentValue, nil
}

// GetCadenceComponentVersionUpgrade returns the most recent upgrade for the Cadence component version,
// if one exists (otherwise returns nil).
func (model *Modelv2) GetCadenceComponentVersionUpgrade() *protocol.ViewBasedActivator[protocol.MagnitudeOfChangeVersion] {
	return model.CadenceComponentVersion.Update
}

// GetExecutionComponentVersion returns the current Execution component version from Modelv2.
// Returns kvstore.ErrKeyNotSet if the key has no value
func (model *Modelv2) GetExecutionComponentVersion() (protocol.MagnitudeOfChangeVersion, error) {
	if model.ExecutionComponentVersion.CurrentValue == protocol.UndefinedMagnitudeOfChangeVersion {
		return protocol.UndefinedMagnitudeOfChangeVersion, ErrKeyNotSet
	}
	return model.ExecutionComponentVersion.CurrentValue, nil
}

// GetExecutionComponentVersionUpgrade returns the most recent upgrade for the Execution component version,
// if one exists (otherwise returns nil).
func (model *Modelv2) GetExecutionComponentVersionUpgrade() *protocol.ViewBasedActivator[protocol.MagnitudeOfChangeVersion] {
	return model.ExecutionComponentVersion.Update
}

// GetExecutionMeteringParameters returns the current Execution metering parameters from Modelv2.
// Returns kvstore.ErrKeyNotSet if the key has no value
func (model *Modelv2) GetExecutionMeteringParameters() (protocol.ExecutionMeteringParameters, error) {
	if model.ExecutionMeteringParameters.CurrentValue.IsUndefined() {
		return protocol.UndefinedExecutionMeteringParameters, ErrKeyNotSet
	}
	return model.ExecutionMeteringParameters.CurrentValue, nil
}

// GetExecutionMeteringParametersUpgrade returns the most recent upgrade for the Execution metering parameters,
// if one exists (otherwise returns nil).
func (model *Modelv2) GetExecutionMeteringParametersUpgrade() *protocol.ViewBasedActivator[protocol.ExecutionMeteringParameters] {
	return model.ExecutionMeteringParameters.Update
}

// NewDefaultKVStore constructs a default Key-Value Store of the *latest* protocol version for bootstrapping.
// Currently, the KV store is largely empty.
// TODO: Shortcut in bootstrapping; we will probably have to start with a non-empty KV store in the future;
// TODO(efm-recovery): we need to bootstrap with v1 in order to test the upgrade to v2. Afterward, we should bootstrap with v2 by default for new networks.
// Potentially we may need to carry over the KVStore during a spork (with possible migrations).
func NewDefaultKVStore(finalizationSafetyThreshold, epochExtensionViewCount uint64, epochStateID flow.Identifier) (protocol_state.KVStoreAPI, error) {
	modelv0, err := newKVStoreV0(finalizationSafetyThreshold, epochExtensionViewCount, epochStateID)
	if err != nil {
		return nil, fmt.Errorf("could not construct v0 kvstore: %w", err)
	}
	return &Modelv1{
		Modelv0: *modelv0,
	}, nil
}

// NewKVStoreV0 constructs a KVStore using the v0 model. This is used to test
// version upgrades, from v0 to v1.
func newKVStoreV0(finalizationSafetyThreshold, epochExtensionViewCount uint64, epochStateID flow.Identifier) (*Modelv0, error) {
	model := &Modelv0{
		UpgradableModel:             UpgradableModel{},
		EpochStateID:                epochStateID,
		FinalizationSafetyThreshold: finalizationSafetyThreshold,
	}
	// use a setter to ensure the default value is valid and is not accidentally lower than the safety threshold.
	err := model.SetEpochExtensionViewCount(epochExtensionViewCount)
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not set default epoch extension view count: %s", err.Error())
	}
	return model, nil
}

// NewKVStoreV0 constructs a KVStore using the v0 model. This is used to test
// version upgrades, from v0 to v1.
func NewKVStoreV0(finalizationSafetyThreshold, epochExtensionViewCount uint64, epochStateID flow.Identifier) (protocol_state.KVStoreAPI, error) {
	return newKVStoreV0(finalizationSafetyThreshold, epochExtensionViewCount, epochStateID)
}

// versionedModel generically represents a versioned protocol state model.
type versionedModel interface {
	GetProtocolStateVersion() uint64
	*Modelv0 | *Modelv1 | *Modelv2
}

// makeVersionedModelID produces an Identifier which includes both the model's
// internal fields and its version. This guarantees that two models with different
// versions but otherwise identical fields will have different IDs, a requirement
// of the protocol.KVStoreReader API.
func makeVersionedModelID[T versionedModel](model T) flow.Identifier {
	return flow.MakeID(struct {
		Version uint64
		Model   T
	}{
		Version: model.GetProtocolStateVersion(),
		Model:   model,
	})
}
