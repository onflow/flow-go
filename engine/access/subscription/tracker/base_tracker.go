package tracker

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// BaseTracker is an interface for a tracker that provides base GetStartHeight method related to both blocks and execution data tracking.
type BaseTracker interface {
	// GetStartHeightFromBlockID returns the start height based on the provided starting block ID.
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
	GetStartHeightFromBlockID(flow.Identifier) (uint64, error)

	// GetStartHeightFromHeight returns the start height based on the provided starting block height.
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
	GetStartHeightFromHeight(uint64) (uint64, error)

	// GetStartHeightFromLatest returns the start height based on the latest sealed block.
	//
	// Parameters:
	// - ctx: Context for the operation.
	//
	// No errors are expected during normal operation.
	GetStartHeightFromLatest(context.Context) (uint64, error)
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

// GetStartHeightFromBlockID returns the start height based on the provided starting block ID.
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
func (b *BaseTrackerImpl) GetStartHeightFromBlockID(startBlockID flow.Identifier) (uint64, error) {
	header, err := b.headers.ByBlockID(startBlockID)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for block %v: %w", startBlockID, err))
	}

	// ensure that the resolved start height is available
	return header.Height, nil
}

// GetStartHeightFromHeight returns the start height based on the provided starting block height.
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
func (b *BaseTrackerImpl) GetStartHeightFromHeight(startHeight uint64) (uint64, error) {
	if startHeight < b.rootBlockHeight {
		return 0, status.Errorf(codes.InvalidArgument, "start height must be greater than or equal to the root height %d", b.rootBlockHeight)
	}

	header, err := b.headers.ByHeight(startHeight)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for height %d: %w", startHeight, err))
	}

	// ensure that the resolved start height is available
	return header.Height, nil
}

// GetStartHeightFromLatest returns the start height based on the latest sealed block.
//
// Parameters:
// - ctx: Context for the operation.
//
// No errors are expected during normal operation.
func (b *BaseTrackerImpl) GetStartHeightFromLatest(ctx context.Context) (uint64, error) {
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
