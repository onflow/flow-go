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
	headers          storage.Headers
	results          storage.ExecutionResults
	kvStoreSnapshots storage.ProtocolKVStore
	kvStoreFactories []protocol_state.KeyValueStoreStateMachineFactory
}

var _ protocol.MutableProtocolState = (*MutableProtocolState)(nil)

// NewMutableProtocolState creates a new instance of MutableProtocolState.
func NewMutableProtocolState(
	protocolStateDB storage.ProtocolState,
	kvStoreSnapshots storage.ProtocolKVStore,
	globalParams protocol.GlobalParams,
	headers storage.Headers,
	results storage.ExecutionResults,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
) *MutableProtocolState {
	kvStoreFactories := []protocol_state.KeyValueStoreStateMachineFactory{
		kvstore.NewPSVersionUpgradeStateMachineFactory(globalParams),
		epochs.NewEpochStateMachineFactory(globalParams, setups, commits, protocolStateDB),
	}
	return &MutableProtocolState{
		ProtocolState:    *NewProtocolState(protocolStateDB, globalParams),
		headers:          headers,
		results:          results,
		kvStoreSnapshots: kvStoreSnapshots,
		kvStoreFactories: kvStoreFactories,
	}
}

// Mutator instantiates a `protocol.StateMutator` based on the previous protocol state.
// Has to be called for each block to evolve the protocol state.
// Expected errors during normal operations:
//   - `storage.ErrNotFound` if no protocol state for parent block is known.
func (s *MutableProtocolState) Mutator(candidate *flow.Header) (protocol.StateMutator, error) {
	parentStateData, err := s.kvStoreSnapshots.ByBlockID(candidate.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not query parent KV store at block (%x): %w", candidate.ParentID, err)
	}
	parentState, err := kvstore.VersionedDecode(parentStateData.Version, parentStateData.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decode parent protocol state (version=%d) at block (%x): %w",
			parentStateData.Version, candidate.ParentID, err)
	}
	return newStateMutator(
		s.headers,
		s.results,
		s.kvStoreSnapshots,
		candidate,
		parentState,
		s.kvStoreFactories...,
	)
}
