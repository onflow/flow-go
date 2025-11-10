package blocks

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// BlocksBase provides shared functionality for block status determination
type BlocksBase struct {
	blocks  storage.Blocks
	headers storage.Headers
	state   protocol.State
}

// NewBlockBase creates a new BlockBase instance
func NewBlockBase(blocks storage.Blocks, headers storage.Headers, state protocol.State) BlocksBase {
	return BlocksBase{
		blocks:  blocks,
		headers: headers,
		state:   state,
	}
}

// getBlockStatus returns the block status for a given header.
//
// No errors are expected during normal operations.
func (b *BlocksBase) getBlockStatus(header *flow.Header) (flow.BlockStatus, error) {
	// check which block is finalized at the target block's height
	// note: this index is only populated for finalized blocks
	blockIDFinalizedAtHeight, err := b.headers.BlockIDByHeight(header.Height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return flow.BlockStatusUnknown, nil // height not indexed yet (not finalized)
		}
		return flow.BlockStatusUnknown, fmt.Errorf("failed to lookup block ID by height: %w", err)
	}

	if blockIDFinalizedAtHeight != header.ID() {
		// The queried block has been orphaned. It will never be finalized or sealed.
		return flow.BlockStatusUnknown, nil
	}

	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return flow.BlockStatusUnknown, fmt.Errorf("failed to lookup sealed header: %w", err)
	}

	if header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}

	return flow.BlockStatusSealed, nil
}

// GetLatestBlock returns the latest block in the chain.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
func (b *BlocksBase) GetLatestBlock(
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
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError]: No block with the given ID was found
func (b *BlocksBase) GetBlockByID(
	ctx context.Context,
	id flow.Identifier,
) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByID(id)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("failed to find block by ID: %w", err)
		return nil, flow.BlockStatusUnknown, access.NewDataNotFoundError("block", err)
	}

	status, err := b.getBlockStatus(block.ToHeader())
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
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError]: No block with the given height was found
func (b *BlocksBase) GetBlockByHeight(
	ctx context.Context,
	height uint64,
) (*flow.Block, flow.BlockStatus, error) {
	block, err := b.blocks.ByHeight(height)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = common.ResolveHeightError(b.state.Params(), height, err)
		err = fmt.Errorf("failed to find block by height: %w", err)
		return nil, flow.BlockStatusUnknown, access.NewDataNotFoundError("block", err)
	}

	status, err := b.getBlockStatus(block.ToHeader())
	if err != nil {
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}

	return block, status, nil
}

// GetLatestBlockHeader returns the latest block header in the chain.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
func (b *BlocksBase) GetLatestBlockHeader(
	ctx context.Context,
	isSealed bool,
) (*flow.Header, flow.BlockStatus, error) {
	if isSealed {
		header, err := b.state.Sealed().Head()
		if err != nil {
			// sealed header must exist in the db, otherwise the node's state may be corrupt
			err = fmt.Errorf("failed to lookup latest sealed header: %w", err)
			return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
		}
		return header, flow.BlockStatusSealed, nil
	}

	header, err := b.state.Final().Head()
	if err != nil {
		// finalized header must exist in the db, otherwise the node's state may be corrupt
		err = fmt.Errorf("failed to find latest finalized header: %w", err)
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}

	// Note: there is a corner case when requesting the latest finalized block before the
	// consensus follower has progressed past the spork root block. In this case, the returned
	// blockStatus will be finalized, however, the block is actually sealed.
	if header.Height == b.state.Params().SporkRootBlockHeight() {
		return header, flow.BlockStatusSealed, nil
	} else {
		return header, flow.BlockStatusFinalized, nil
	}
}

// GetBlockHeaderByID returns the block header with the given ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError]: No header with the given ID was found
func (b *BlocksBase) GetBlockHeaderByID(
	ctx context.Context,
	id flow.Identifier,
) (*flow.Header, flow.BlockStatus, error) {
	header, err := b.headers.ByBlockID(id)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("failed to find header by ID: %w", err)
		return nil, flow.BlockStatusUnknown, access.NewDataNotFoundError("header", err)
	}

	status, err := b.getBlockStatus(header)
	if err != nil {
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}
	return header, status, nil
}

// GetBlockHeaderByHeight returns the block header at the given height.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError]: No header with the given height was found
func (b *BlocksBase) GetBlockHeaderByHeight(
	ctx context.Context,
	height uint64,
) (*flow.Header, flow.BlockStatus, error) {
	header, err := b.headers.ByHeight(height)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = common.ResolveHeightError(b.state.Params(), height, err)
		err = fmt.Errorf("failed to find header by height: %w", err)
		return nil, flow.BlockStatusUnknown, access.NewDataNotFoundError("header", err)
	}

	status, err := b.getBlockStatus(header)
	if err != nil {
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}
	return header, status, nil
}
