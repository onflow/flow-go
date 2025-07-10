package backend

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendBlockHeaders struct {
	headers storage.Headers
	state   protocol.State
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
	return header, flow.BlockStatusFinalized, nil
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

	stat, err := b.getBlockStatus(header)
	if err != nil {
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}
	return header, stat, nil
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
		err = resolveHeightError(b.state.Params(), height, err)
		err = fmt.Errorf("failed to find header by height: %w", err)
		return nil, flow.BlockStatusUnknown, access.NewDataNotFoundError("header", err)
	}

	stat, err := b.getBlockStatus(header)
	if err != nil {
		return nil, flow.BlockStatusUnknown, access.RequireNoError(ctx, err)
	}
	return header, stat, nil
}

// getBlockStatus returns the status of the block
//
// No errors are expected during normal operations.
func (b *backendBlockHeaders) getBlockStatus(header *flow.Header) (flow.BlockStatus, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		// sealed header must exist in the db, otherwise the node's state may be corrupt
		return flow.BlockStatusUnknown, fmt.Errorf("failed to lookup latest sealed header: %w", err)
	}

	if header.Height > sealed.Height {
		return flow.BlockStatusFinalized, nil
	}
	return flow.BlockStatusSealed, nil
}
