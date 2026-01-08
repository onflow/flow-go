package execution_result

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// SealingStatusResolver provides an abstraction for checking the sealing status of blocks.
// This encapsulates the knowledge of how a block is determined to be sealed.
type SealingStatusResolver interface {
	// IsSealed returns true if the block with the given ID is already sealed.
	IsSealed(blockID flow.Identifier) (bool, error)
}

// SealingStatusResolverImpl implements SealingStatusResolver using the protocol state.
// This implementation properly encapsulates the logic for determining if a block is sealed
// without leaking low-level height comparisons into the domain logic.
type SealingStatusResolverImpl struct {
	headers storage.Headers
	state   protocol.State
}

// NewSealingStatusResolver creates a new SealingStatusResolverImpl.
func NewSealingStatusResolver(headers storage.Headers, state protocol.State) *SealingStatusResolverImpl {
	return &SealingStatusResolverImpl{
		headers: headers,
		state:   state,
	}
}

// IsSealed checks if a block is sealed.
func (s *SealingStatusResolverImpl) IsSealed(blockID flow.Identifier) (bool, error) {
	// Step 1: Get the block header to know its height
	header, err := s.headers.ByBlockID(blockID)
	if err != nil {
		return false, fmt.Errorf("failed to get header for block %s: %w", blockID, err)
	}

	// Step 2: Check if this block is finalized.
	// Only finalized blocks can be sealed.
	finalizedBlockID, err := s.headers.BlockIDByHeight(header.Height)
	if err != nil {
		// No finalized block at this height means it's definitely not sealed.
		return false, nil
	}

	if finalizedBlockID != blockID {
		// Different block finalized at this height means this one is not finalized and thus not sealed.
		return false, nil
	}

	// Step 3: Check against the current sealed head.
	// This uses the protocol state's knowledge of the latest sealed block.
	sealedHeader, err := s.state.Sealed().Head()
	if err != nil {
		return false, fmt.Errorf("failed to retrieve sealed head: %w", err)
	}

	return header.Height <= sealedHeader.Height, nil
}
