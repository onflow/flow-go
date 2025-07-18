package backend

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// backendBlockBase provides shared functionality for block status determination
type backendBlockBase struct {
	blocks  storage.Blocks
	headers storage.Headers
	state   protocol.State
}

// Expected errors during normal operations:
// - storage.ErrNotFound - Indicates that the queried block height is not finalized yet.
func (b *backendBlockBase) getBlockStatus(header *flow.Header) (flow.BlockStatus, error) {
	blockIDFinalizedAtHeight, err := b.headers.BlockIDByHeight(header.Height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return flow.BlockStatusUnknown, nil // height not indexed yet (not finalized)
		}
		return flow.BlockStatusUnknown, fmt.Errorf("failed to lookup block ID by height: %w", err)
	}

	if blockIDFinalizedAtHeight != header.ID() {
		// A different block than what was queried has been finalized at this height.
		return flow.BlockStatusUnknown, nil
	}

	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return flow.BlockStatusUnknown, err
	}

	if header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}

	return flow.BlockStatusSealed, nil
}
