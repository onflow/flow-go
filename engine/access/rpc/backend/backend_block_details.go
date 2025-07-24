package backend

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type backendBlockDetails struct {
	backendBlockBase
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
	var blockStatus flow.BlockStatus
	var err error

	if isSealed {
		header, err = b.state.Sealed().Head()
		if err != nil {
			// sealed header must exist in the db, otherwise the node's state may be corrupt
			err = fmt.Errorf("failed to lookup latest sealed header: %w", err)
			return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
		}
		blockStatus = flow.BlockStatusSealed
	} else {
		header, err = b.state.Final().Head()
		if err != nil {
			// finalized header must exist in the db, otherwise the node's state may be corrupt
			err = fmt.Errorf("failed to lookup latest finalize header: %w", err)
			return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
		}
		blockStatus = flow.BlockStatusFinalized

		// Note: there is a corner case when requesting the latest finalized block before the
		// consensus follower has progressed past the spork root block. In this case, the returned
		// blockStatus will be finalized, however, the block is actually sealed.
		if header.Height == b.state.Params().SporkRootBlockHeight() {
			blockStatus = flow.BlockStatusSealed
		} else {
			blockStatus = flow.BlockStatusFinalized
		}
	}

	// since we are querying a finalized or sealed block, we can use the height index and save an ID computation
	block, err := b.blocks.ByHeight(header.Height)
	if err != nil {
		// finalized and sealed blocks must exist in the db, otherwise the node's state may be corrupt
		err = fmt.Errorf("could not get latest block: %w", err)
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}

	return block, blockStatus, nil
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

	status, err := b.getBlockStatus(block.Header)
	if err != nil {
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}
	return block, status, nil
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

	status, err := b.getBlockStatus(block.Header)
	if err != nil {
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}
	return block, status, nil
}
