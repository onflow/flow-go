package protocol_state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ProtocolState is an implementation of the read-only interface for protocol state, it allows to query information
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
	return newDynamicProtocolStateAdapter(protocolStateEntry, s.globalParams), nil
}

// GlobalParams returns an interface which can be used to query global protocol parameters.
func (s *ProtocolState) GlobalParams() protocol.GlobalParams {
	return s.globalParams
}
