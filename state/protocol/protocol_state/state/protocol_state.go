package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/epochs"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/storage"
)

// ProtocolState is an implementation of the read-only interface for protocol state, it allows querying information
// on a per-block and per-epoch basis.
// It is backed by a storage.ProtocolState and an in-memory protocol.GlobalParams.
type ProtocolState struct {
	epochProtocolStateDB storage.ProtocolState
	kvStoreSnapshots     storage.ProtocolKVStore
	globalParams         protocol.GlobalParams
}

var _ protocol.ProtocolState = (*ProtocolState)(nil)

func NewProtocolState(protocolStateDB storage.ProtocolState, kvStoreSnapshots storage.ProtocolKVStore, globalParams protocol.GlobalParams) *ProtocolState {
	return &ProtocolState{
		epochProtocolStateDB: protocolStateDB,
		kvStoreSnapshots:     kvStoreSnapshots,
		globalParams:         globalParams,
	}
}

// AtBlockID returns epoch protocol state at block ID.
// The resulting epoch protocol state is returned AFTER applying updates that are contained in block.
// Can be queried for any block that has been added to the block tree.
// Returns:
// - (DynamicProtocolState, nil) - if there is an epoch protocol state associated with given block ID.
// - (nil, storage.ErrNotFound) - if there is no epoch protocol state associated with given block ID.
// - (nil, exception) - any other error should be treated as exception.
func (s *ProtocolState) AtBlockID(blockID flow.Identifier) (protocol.DynamicProtocolState, error) {
	protocolStateEntry, err := s.epochProtocolStateDB.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not query epoch protocol state at block (%x): %w", blockID, err)
	}
	return inmem.NewDynamicProtocolStateAdapter(protocolStateEntry, s.globalParams), nil
}

// KVStoreAtBlockID returns protocol state at block ID.
// The resulting protocol state is returned AFTER applying updates that are contained in block.
// Can be queried for any block that has been added to the block tree.
// Returns:
// - (KVStoreReader, nil) - if there is a protocol state associated with given block ID.
// - (nil, storage.ErrNotFound) - if there is no protocol state associated with given block ID.
// - (nil, exception) - any other error should be treated as exception.
func (s *ProtocolState) KVStoreAtBlockID(blockID flow.Identifier) (protocol.KVStoreReader, error) {
	return s.kvStoreAtBlockID(blockID)
}

// kvStoreAtBlockID queries KV store by block ID and decodes it from binary data to a typed interface.
// Returns:
// - (protocol_state.KVStoreAPI, nil) - if there is a protocol state associated with given block ID.
// - (nil, storage.ErrNotFound) - if there is no protocol state associated with given block ID.
// - (nil, exception) - any other error should be treated as exception.
func (s *ProtocolState) kvStoreAtBlockID(blockID flow.Identifier) (protocol_state.KVStoreAPI, error) {
	versionedData, err := s.kvStoreSnapshots.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not query KV store at block (%x): %w", blockID, err)
	}
	kvStore, err := kvstore.VersionedDecode(versionedData.Version, versionedData.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decode protocol state (version=%d) at block (%x): %w",
			versionedData.Version, blockID, err)
	}
	return kvStore, err
}

// GlobalParams returns an interface which can be used to query global protocol parameters.
func (s *ProtocolState) GlobalParams() protocol.GlobalParams {
	return s.globalParams
}

// MutableProtocolState is an implementation of the mutable interface for protocol state, it allows to evolve the protocol state
// by acting as factory for protocol.StateMutator which can be used to apply state-changing operations.
type MutableProtocolState struct {
	ProtocolState
	headers          storage.Headers
	results          storage.ExecutionResults
	kvStoreFactories []protocol_state.KeyValueStoreStateMachineFactory
}

var _ protocol.MutableProtocolState = (*MutableProtocolState)(nil)

// NewMutableProtocolState creates a new instance of MutableProtocolState.
func NewMutableProtocolState(
	epochProtocolStateDB storage.ProtocolState,
	kvStoreSnapshots storage.ProtocolKVStore,
	globalParams protocol.GlobalParams,
	headers storage.Headers,
	results storage.ExecutionResults,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
) *MutableProtocolState {
	// an ordered list of factories to create state machines for different sub-states of the Dynamic Protocol State.
	// all factories are expected to be called in order defined here.
	kvStoreFactories := []protocol_state.KeyValueStoreStateMachineFactory{
		kvstore.NewPSVersionUpgradeStateMachineFactory(globalParams),
		epochs.NewEpochStateMachineFactory(globalParams, setups, commits, epochProtocolStateDB),
	}
	return &MutableProtocolState{
		ProtocolState:    *NewProtocolState(epochProtocolStateDB, kvStoreSnapshots, globalParams),
		headers:          headers,
		results:          results,
		kvStoreFactories: kvStoreFactories,
	}
}

// Mutator instantiates a `protocol.StateMutator` based on the previous protocol state.
// Has to be called for each block to evolve the protocol state.
// Expected errors during normal operations:
//   - `storage.ErrNotFound` if no protocol state for parent block is known.
func (s *MutableProtocolState) Mutator(candidateView uint64, parentID flow.Identifier) (protocol.StateMutator, error) {
	parentState, err := s.kvStoreAtBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not query kvstore at parent block: %w", err)
	}
	return newStateMutator(
		s.headers,
		s.results,
		s.kvStoreSnapshots,
		candidateView,
		parentID,
		parentState,
		s.kvStoreFactories...,
	)
}
