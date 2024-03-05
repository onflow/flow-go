package subscription

import (
	"context"
	"fmt"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// GetStartHeightFunc is a function type for getting the start height.
type GetStartHeightFunc func(context.Context, flow.Identifier, uint64) (uint64, error)

// StreamingData represents common streaming data configuration for access and state_stream handlers.
type StreamingData struct {
	MaxStreams  int32
	StreamCount atomic.Int32
}

func NewStreamingData(maxStreams uint32) StreamingData {
	return StreamingData{
		MaxStreams:  int32(maxStreams),
		StreamCount: atomic.Int32{},
	}
}

// BaseTracker is an interface for a tracker that provides base GetStartHeight method related to both blocks and execution data tracking.
type BaseTracker interface {
	// GetStartHeight returns the start height to use when searching.
	// Only one of startBlockID and startHeight may be set. Otherwise, an InvalidArgument error is returned.
	// If a block is provided and does not exist, a NotFound error is returned.
	// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
	// If the start block is the root block, skip it and begin from the next block.
	//
	// Expected errors during normal operation:
	// - codes.InvalidArgument - if both startBlockID and startHeight are provided, if the start height is less than the root block height.
	// - storage.ErrNotFound   - if a block is provided and does not exist.
	// - codes.Internal        - if there is an internal error.
	GetStartHeight(context.Context, flow.Identifier, uint64) (uint64, error)
}

var _ BaseTracker = (*BaseTrackerImpl)(nil)

// BaseTrackerImpl is an implementation of the BaseTracker interface.
type BaseTrackerImpl struct {
	rootBlockHeight uint64
	state           protocol.State
	headers         storage.Headers
}

// NewBaseTrackerImpl creates a new instance of BaseTrackerImpl.
//
// Parameters:
// - rootBlockHeight: The root block height, which serves as the baseline for calculating the start height.
// - state: The protocol state used for retrieving block information.
// - headers: The storage headers for accessing block headers.
//
// Returns:
// - *BaseTrackerImpl: A new instance of BaseTrackerImpl.
func NewBaseTrackerImpl(
	rootBlockHeight uint64,
	state protocol.State,
	headers storage.Headers,
) *BaseTrackerImpl {
	return &BaseTrackerImpl{
		rootBlockHeight: rootBlockHeight,
		state:           state,
		headers:         headers,
	}
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
// - codes.InvalidArgument - if both startBlockID and startHeight are provided, if the start height is less than the root block height.
// - storage.ErrNotFound   - if a block is provided and does not exist.
// - codes.Internal        - if there is an internal error.
func (b *BaseTrackerImpl) GetStartHeight(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
	height, err := b.getHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return 0, err
	}

	// ensure that the resolved start height is available
	return b.checkStartHeight(height), nil
}

// getHeight returns the block height based on initial data without general validate.
// Only one of startBlockID and startHeight may be set. Otherwise, an InvalidArgument error is returned.
// If a block is provided and does not exist, a NotFound error is returned.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
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
// - codes.InvalidArgument - if both startBlockID and startHeight are provided, if the start height is less than the root block height.
// - storage.ErrNotFound   - if a block is provided and does not exist.
// - codes.Internal        - if there is an internal error.
func (b *BaseTrackerImpl) getHeight(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
	// make sure only one of start block ID and start height is provided
	if startBlockID != flow.ZeroID && startHeight > 0 {
		return 0, status.Errorf(codes.InvalidArgument, "only one of start block ID and start height may be provided")
	}

	if startBlockID != flow.ZeroID {
		return b.startHeightFromBlockID(startBlockID)
	}

	if startHeight > 0 {
		return b.startHeightFromHeight(startHeight)
	}

	// if no start block was provided, use the latest sealed block
	header, err := b.state.Sealed().Head()
	if err != nil {
		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return 0, err
	}

	return header.Height, nil
}

// checkStartHeight validates the provided start height and adjusts it if necessary based on the tracker's configuration.
// If the start block is the root block, skip it and begin from the next block.
//
// Parameters:
// - height: The start height to be checked.
//
// Returns:
// - uint64: The adjusted start height.
//
// No errors are expected during normal operation.
func (b *BaseTrackerImpl) checkStartHeight(height uint64) uint64 {
	// if the start block is the root block, skip it and begin from the next block.
	if height == b.rootBlockHeight {
		height = b.rootBlockHeight + 1
	}

	return height
}

// startHeightFromBlockID returns the start height based on the provided starting block ID.
//
// Parameters:
// - startBlockID: The identifier of the starting block
//
// Returns:
// - uint64: The start height associated with the provided block ID.
// - error: An error indicating any issues with retrieving the start height.
//
// Expected errors during normal operation:
// - rpc.ConvertStorageError - if there is an issue retrieving the header from the storage.
func (b *BaseTrackerImpl) startHeightFromBlockID(startBlockID flow.Identifier) (uint64, error) {
	header, err := b.headers.ByBlockID(startBlockID)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for block %v: %w", startBlockID, err))
	}
	return header.Height, nil
}

// startHeightFromHeight returns the start height based on the provided starting block height.
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
// - rpc.ConvertStorageError - if there is an issue retrieving the header from the storage.
func (b *BaseTrackerImpl) startHeightFromHeight(startHeight uint64) (uint64, error) {
	if startHeight < b.rootBlockHeight {
		return 0, status.Errorf(codes.InvalidArgument, "start height must be greater than or equal to the root height %d", b.rootBlockHeight)
	}

	header, err := b.headers.ByHeight(startHeight)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for height %d: %w", startHeight, err))
	}
	return header.Height, nil
}
