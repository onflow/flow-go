package protocol_state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
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
	headers storage.Headers
	results storage.ExecutionResults
	setups  storage.EpochSetups
	commits storage.EpochCommits
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
) *MutableProtocolState {
	return &MutableProtocolState{
		ProtocolState: *NewProtocolState(protocolStateDB, globalParams),
		headers:       headers,
		results:       results,
		setups:        setups,
		commits:       commits,
	}
}

// TODO rename to Mutator factory

// Mutator instantiates a `protocol.StateMutator` based on the previous protocol state.
// Has to be called for each block to evolve the protocol state.
// Expected errors during normal operations:
//   - `storage.ErrNotFound` if no protocol state for parent block is known.
func (s *MutableProtocolState) Mutator(candidateView uint64, parentID flow.Identifier) (protocol.StateMutator, error) {
	parentState, err := s.protocolStateDB.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not query parent protocol state at block (%x): %w", parentID, err)
	}
	return newStateMutator(
		s.headers,
		s.results,
		s.setups,
		s.commits,
		candidateView,
		parentState,
		NewStateMachine,
		NewEpochFallbackStateMachine,
	)
}
