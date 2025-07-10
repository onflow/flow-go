package backend

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendBlockDetails struct {
	blocks storage.Blocks
	state  protocol.State
}

// GetLatestBlock returns the latest block in the chain.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
func (b *backendBlockDetails) GetLatestBlock(
	ctx context.Context,
	isSealed bool,
) (*flow.Block, flow.BlockStatus, error) {
	var header *flow.Header
	var status flow.BlockStatus
	var err error

	if isSealed {
		header, err = b.state.Sealed().Head()
		if err != nil {
			// sealed header must exist in the db, otherwise the node's state may be corrupt
			err = fmt.Errorf("failed to lookup latest sealed header: %w", err)
			return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
		}
		status = flow.BlockStatusSealed
	} else {
		header, err = b.state.Final().Head()
		if err != nil {
			// finalized header must exist in the db, otherwise the node's state may be corrupt
			err = fmt.Errorf("failed to lookup latest finalize header: %w", err)
			return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
		}
		status = flow.BlockStatusFinalized
	}

	// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
	block, err := b.blocks.ByHeight(header.Height)
	if err != nil {
		// finalized and sealed blocks must exist in the db, otherwise the node's state may be corrupt
		err = fmt.Errorf("could not get latest block: %w", err)
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}

	return block, status, nil
}

// GetBlockByID returns the block with the given ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected errors:
//   - access.DataNotFoundError - No block with the given ID was found
func (b *backendBlockDetails) GetBlockByID(
	ctx context.Context,
	id flow.Identifier,
) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByID(id)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("failed to find block by ID: %w", err)
		return nil, flow.BlockStatusUnknown, access.NewDataNotFoundError("block", err)
	}

	stat, err := b.getBlockStatus(block)
	if err != nil {
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}
	return block, stat, nil
}

// GetBlockByHeight returns the block at the given height.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected errors:
//   - access.DataNotFoundError - No block with the given height was found
func (b *backendBlockDetails) GetBlockByHeight(
	ctx context.Context,
	height uint64,
) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = resolveHeightError(b.state.Params(), height, err)
		err = fmt.Errorf("failed to find block by height: %w", err)
		return nil, flow.BlockStatusUnknown, access.NewDataNotFoundError("block", err)
	}

	stat, err := b.getBlockStatus(block)
	if err != nil {
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}
	return block, stat, nil
}

// getBlockStatus returns the status of the block
//
// No errors are expected during normal operations.
func (b *backendBlockDetails) getBlockStatus(block *flow.Block) (flow.BlockStatus, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		// sealed header must exist in the db, otherwise the node's state may be corrupt
		return flow.BlockStatusUnknown, fmt.Errorf("failed to lookup latest sealed header: %w", err)
	}

	if block.Header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}
	return flow.BlockStatusSealed, nil
}
