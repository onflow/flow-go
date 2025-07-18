package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// backendBlockBase provides shared functionality for block status determination
type backendBlockBase struct {
	blocks  storage.Blocks
	headers storage.Headers
	state   protocol.State
}

//
// Expected errors during normal operations:
func (b *backendBlockBase) getBlockStatus(ctx context.Context, header *flow.Header) (flow.BlockStatus, error) {
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
		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests, but do throw an exception which might cause a crash:
		// - It is unsafe to process requests if we have an internally bad State.
		// - We would like to avoid throwing an exception as a result of an Access API request by policy
		//   because this can cause DOS potential
		// - Since the protocol state is widely shared, we assume that in practice another component will
		//   observe the protocol state error and throw an exception.
		err := irrecoverable.NewExceptionf("failed to get sealed head: %w", err)
		irrecoverable.Throw(ctx, err)
		return flow.BlockStatusUnknown, err
	}

	if header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}

	return flow.BlockStatusSealed, nil
}
