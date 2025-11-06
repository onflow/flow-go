package tracker

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// HeightTracker is an implementation of the BaseTracker interface.
type HeightTracker struct {
	rootBlockHeight uint64
	state           protocol.State
	headers         storage.Headers
}

var _ subscription.HeightTracker = (*HeightTracker)(nil)

// NewHeightTracker creates a new instance of HeightTracker.
//
// Parameters:
// - rootBlockHeight: The root block height, which serves as the baseline for calculating the start height.
// - state: The protocol state used for retrieving block information.
// - headers: The storage headers for accessing block headers.
//
// Returns:
// - *HeightTracker: A new instance of HeightTracker.
func NewHeightTracker(
	rootBlockHeight uint64,
	state protocol.State,
	headers storage.Headers,
) *HeightTracker {
	return &HeightTracker{
		rootBlockHeight: rootBlockHeight,
		state:           state,
		headers:         headers,
	}
}

// GetStartHeightFromBlockID returns the start height based on the provided starting block ID.
// If the start block is the root block, skip it and begin from the next block.
//
// Parameters:
// - startBlockID: The identifier of the starting block.
//
// Returns:
// - uint64: The start height associated with the provided block ID.
// - error: An error indicating any issues with retrieving the start height.
//
// Expected errors during normal operation:
// - codes.NotFound - if the block was not found in storage
// - codes.Internal - for any other error
func (b *HeightTracker) GetStartHeightFromBlockID(startBlockID flow.Identifier) (uint64, error) {
	header, err := b.headers.ByBlockID(startBlockID)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for block %v: %w", startBlockID, err))
	}

	// ensure that the resolved start height is available
	return b.getAdjustedHeight(header.Height), nil
}

// GetStartHeightFromHeight returns the start height based on the provided starting block height.
// If the start block is the root block, skip it and begin from the next block.
//
// Parameters:
// - startHeight: The height of the starting block.
//
// Returns:
// - uint64: The start height associated with the provided block height.
// - error: An error indicating any issues with retrieving the start height.
//
// Expected errors during normal operation:
// - codes.InvalidArgument   - if the start height is less than the root block height.
// - codes.NotFound  - if the header was not found in storage.
func (b *HeightTracker) GetStartHeightFromHeight(startHeight uint64) (uint64, error) {
	if startHeight < b.rootBlockHeight {
		return 0, status.Errorf(codes.InvalidArgument, "start height must be greater than or equal to the root height %d", b.rootBlockHeight)
	}

	header, err := b.headers.ByHeight(startHeight)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for height %d: %w", startHeight, err))
	}

	// ensure that the resolved start height is available
	return b.getAdjustedHeight(header.Height), nil
}

// GetStartHeightFromLatest returns the start height based on the latest sealed block.
// If the start block is the root block, skip it and begin from the next block.
//
// Parameters:
// - ctx: Context for the operation.
//
// No errors are expected during normal operation.
func (b *HeightTracker) GetStartHeightFromLatest(ctx context.Context) (uint64, error) {
	// if no start block was provided, use the latest sealed block
	header, err := b.state.Sealed().Head()
	if err != nil {
		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return 0, err
	}

	return b.getAdjustedHeight(header.Height), nil
}

// getAdjustedHeight validates the provided start height and adjusts it if necessary.
// If the start block is the root block, skip it and begin from the next block.
//
// Parameters:
// - height: The start height to be checked.
//
// Returns:
// - uint64: The adjusted start height.
//
// No errors are expected during normal operation.
func (b *HeightTracker) getAdjustedHeight(height uint64) uint64 {
	// if the start block is the root block, skip it and begin from the next block.
	if height == b.rootBlockHeight {
		height = b.rootBlockHeight + 1
	}

	return height
}

// GetStartHeight returns the start height to use when searching.
// Only one of startBlockID and startHeight may be set. Otherwise, an InvalidArgument error is returned.
// If a block is provided and does not exist, a NotFound error is returned.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
// If the start block is the root block, skip it and begin from the next block.
//
// Parameters:
// - ctx: Context for the operation.
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
//
// Returns:
// - uint64: The start height for searching.
// - error: An error indicating the result of the operation, if any.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if both startBlockID and startHeight are provided, if the start height is less than the root block height,
// if the start height is out of bounds based on indexed heights (when index is used).
// - codes.NotFound        - if a block is provided and does not exist.
// - codes.Internal        - if there is an internal error.
func (b *HeightTracker) GetStartHeight(
	ctx context.Context,
	startBlockID flow.Identifier,
	startHeight uint64,
) (uint64, error) {
	if startBlockID != flow.ZeroID && startHeight > 0 {
		return 0, status.Errorf(codes.InvalidArgument, "only one of start block ID and start height may be provided")
	}

	// get the start height based on the provided starting block ID
	if startBlockID != flow.ZeroID {
		return b.GetStartHeightFromBlockID(startBlockID)
	}

	// get start height based on the provided starting block height
	if startHeight > 0 {
		return b.GetStartHeightFromHeight(startHeight)
	}

	return b.GetStartHeightFromLatest(ctx)
}
