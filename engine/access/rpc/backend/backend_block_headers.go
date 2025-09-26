package backend

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type backendBlockHeaders struct {
	backendBlockBase
}

// GetLatestBlockHeader returns the latest block header in the chain.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
func (b *backendBlockHeaders) GetLatestBlockHeader(
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
// Expected errors:
//   - access.DataNotFoundError - No header with the given ID was found
func (b *backendBlockHeaders) GetBlockHeaderByID(
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
// Expected errors:
//   - access.DataNotFoundError - No header with the given height was found
func (b *backendBlockHeaders) GetBlockHeaderByHeight(
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
