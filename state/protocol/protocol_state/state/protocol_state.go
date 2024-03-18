package state

import (
	"fmt"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs"
	"github.com/onflow/flow-go/storage"
)

// ProtocolState is an implementation of the read-only interface for protocol state, it allows querying information
// on a per-block and per-epoch basis.
// It is backed by a storage.ProtocolState and an in-memory protocol.GlobalParams.
type ProtocolState struct {
	protocolStateDB storage.ProtocolState
	globalParams    protocol.GlobalParams
}

var _ protocol.ProtocolState = (*ProtocolState)(nil)

func NewProtocolState(protocolStateDB storage.ProtocolState, globalParams protocol.GlobalParams) *ProtocolState {
	return &ProtocolState{
		protocolStateDB: protocolStateDB,
		globalParams:    globalParams,
	}
}

// AtBlockID returns protocol state at block ID.
// Resulting protocol state is returned AFTER applying updates that are contained in block.
// Returns:
// - (DynamicProtocolState, nil) - if there is a protocol state associated with given block ID.
// - (nil, storage.ErrNotFound) - if there is no protocol state associated with given block ID.
// - (nil, exception) - any other error should be treated as exception.
func (s *ProtocolState) AtBlockID(blockID flow.Identifier) (protocol.DynamicProtocolState, error) {
	protocolStateEntry, err := s.protocolStateDB.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not query protocol state at block (%x): %w", blockID, err)
	}
	return inmem.NewDynamicProtocolStateAdapter(protocolStateEntry, s.globalParams), nil
}

// GlobalParams returns an interface which can be used to query global protocol parameters.
func (s *ProtocolState) GlobalParams() protocol.GlobalParams {
	return s.globalParams
}

// MutableProtocolState is an implementation of the mutable interface for protocol state, it allows to evolve the protocol state
// by acting as factory for protocol.StateMutator which can be used to apply state-changing operations.
type MutableProtocolState struct {
	ProtocolState
	headers           storage.Headers
	results           storage.ExecutionResults
	setups            storage.EpochSetups
	commits           storage.EpochCommits
	kvStoreSnapshotDB storage.ProtocolKVStore
}

var _ protocol.MutableProtocolState = (*MutableProtocolState)(nil)

// NewMutableProtocolState creates a new instance of MutableProtocolState.
func NewMutableProtocolState(
	protocolStateDB storage.ProtocolState,
	globalParams protocol.GlobalParams,
	headers storage.Headers,
	results storage.ExecutionResults,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	kvStoreSnapshotDB storage.ProtocolKVStore,
) *MutableProtocolState {
	return &MutableProtocolState{
		ProtocolState:     *NewProtocolState(protocolStateDB, globalParams),
		headers:           headers,
		results:           results,
		setups:            setups,
		commits:           commits,
		kvStoreSnapshotDB: kvStoreSnapshotDB,
	}
}

// Mutator instantiates a `protocol.StateMutator` based on the previous protocol state.
// Has to be called for each block to evolve the protocol state.
// Expected errors during normal operations:
//   - `storage.ErrNotFound` if no protocol state for parent block is known.
func (s *MutableProtocolState) Mutator(candidateView uint64, parentID flow.Identifier) (protocol.StateMutator, error) {
	parentState, err := s.protocolStateDB.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not query parent protocol state at block (%x): %w", parentID, err)
	}

	kvStoreStateMachine, err := s.createKVStoreStateMachine(candidateView, parentID)
	if err != nil {
		return nil, fmt.Errorf("could not create KV store state machine: %w", err)
	}

	return newStateMutator(
		s.headers,
		s.results,
		s.setups,
		s.commits,
		s.globalParams,
		candidateView,
		parentState,
		func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) { // needed for translating from concrete implementation type to interface type
			return epochs.NewStateMachine(candidateView, parentState)
		},
		func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (epochs.ProtocolStateMachine, error) { // needed for translating from concrete implementation type to interface type
			return epochs.NewEpochFallbackStateMachine(candidateView, parentState), nil
		},
		kvStoreStateMachine,
	)
}

func (s *MutableProtocolState) createKVStoreStateMachine(candidateView uint64, parentID flow.Identifier) (protocol_state.KeyValueStoreStateMachine, error) {
	parentKVStore, err := s.retrieveKVStoreSnapshotByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent KV store snapshot at block (%x): %w", parentID, err)
	}

	protocolVersion := parentKVStore.GetProtocolStateVersion()
	if versionUpgrade := parentKVStore.GetVersionUpgrade(); versionUpgrade != nil {
		if candidateView >= versionUpgrade.ActivationView {
			protocolVersion = versionUpgrade.Data
		}
	}
	candidateBlockKVStore, err := parentKVStore.Replicate(protocolVersion)
	if err != nil {
		return nil, fmt.Errorf("could not replicate parent KV store (version=%d) to protocol version %d: %w",
			parentKVStore.GetProtocolStateVersion(), protocolVersion, err)
	}

	return kvstore.NewProcessingStateMachine(candidateView, s.globalParams, parentKVStore, candidateBlockKVStore), nil
}

func (s *MutableProtocolState) retrieveKVStoreSnapshotByBlockID(blockID flow.Identifier) (protocol_state.KVStoreAPI, error) {
	kvStoreData, err := s.kvStoreSnapshotDB.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not query parent KV store snapshot at block (%x): %w", blockID, err)
	}

	kvStore, err := kvstore.VersionedDecode(kvStoreData.Version, kvStoreData.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decode parent KV store (version=%d) snapshot at block (%x): %w",
			kvStoreData.Version, blockID, err)
	}
	return kvStore, nil
}
