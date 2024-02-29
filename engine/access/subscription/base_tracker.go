package subscription

import (
	"context"
	"fmt"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// GetStartHeightFunc is a function type for getting the start height.
type GetStartHeightFunc func(context.Context, flow.Identifier, uint64) (uint64, error)

// GetHighestHeight is a function type for getting the highest height.
type GetHighestHeight func(flow.BlockStatus) (uint64, error)

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
	//
	// Expected errors during normal operation:
	// - codes.InvalidArgument: If both startBlockID and startHeight are provided.
	// - storage.ErrNotFound`: If a block is provided and does not exist.
	// - codes.Internal: If there is an internal error.
	GetStartHeight(context.Context, flow.Identifier, uint64) (uint64, error)
}

// BaseTrackerImpl is an implementation of the BaseTracker interface.
type BaseTrackerImpl struct {
	rootBlockHeight uint64
	state           protocol.State
	headers         storage.Headers

	indexReporter state_synchronization.IndexReporter
	useIndex      bool
}

var _ BaseTracker = (*BaseTrackerImpl)(nil)

// NewBaseTrackerImpl creates a new instance of BaseTrackerImpl.
//
// Parameters:
// - rootBlockHeight: The root block height, which serves as the baseline for calculating the start height.
// - state: The protocol state used for retrieving block information.
// - headers: The storage headers for accessing block headers.
// - indexReporter: The index reporter for checking indexed block heights.
// - useIndex: A flag indicating whether to use indexed block heights for validation.
//
// Returns:
// - *BaseTrackerImpl: A new instance of BaseTrackerImpl.
func NewBaseTrackerImpl(
	rootBlockHeight uint64,
	state protocol.State,
	headers storage.Headers,
	indexReporter state_synchronization.IndexReporter,
	useIndex bool) *BaseTrackerImpl {
	return &BaseTrackerImpl{
		rootBlockHeight: rootBlockHeight,
		state:           state,
		headers:         headers,
		indexReporter:   indexReporter,
		useIndex:        useIndex,
	}
}

// GetStartHeight returns the start height to use when searching.
// Only one of startBlockID and startHeight may be set. Otherwise, an InvalidArgument error is returned.
// If a block is provided and does not exist, a NotFound error is returned.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
//
// Parameters:
// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// Returns:
// - uint64: The start height for searching.
// - error: An error indicating the result of the operation, if any.
//
// Expected errors during normal operation:
// - codes.InvalidArgument: If blockStatus is flow.BlockStatusUnknown, or both startBlockID and startHeight are provided.
// - storage.ErrNotFound`: If a block is provided and does not exist.
// - codes.Internal: If there is an internal error.
func (b *BaseTrackerImpl) GetStartHeight(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) (height uint64, err error) {

	// make sure only one of start block ID and start height is provided
	if startBlockID != flow.ZeroID && startHeight > 0 {
		return 0, status.Errorf(codes.InvalidArgument, "only one of start block ID and start height may be provided")
	}

	// ensure that the resolved start height is available
	defer func() {
		if err == nil {
			height, err = b.checkStartHeight(height)
		}
	}()

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
// - rpc.ConvertStorageError: If there is an issue retrieving the header from the storage.
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
// - codes.InvalidArgument: If the start height is less than the root block height.
// - rpc.ConvertStorageError: If there is an issue retrieving the header from the storage.
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

// checkStartHeight validates the provided start height and adjusts it if necessary based on the tracker's configuration.
//
// Parameters:
// - height: The start height to be checked.
//
// Returns:
// - uint64: The adjusted start height, if validation passes.
// - error: An error indicating any issues with the provided start height.
//
// Validation Steps:
// 1. If the start block is the root block, there won't be execution data. Skip it and begin from the next block.
// 2. If index usage is disabled, return the original height without further checks.
// 3. Retrieve the lowest and highest indexed block heights.
// 4. Check if the provided height is within the bounds of indexed heights.
//   - If below the lowest indexed height, return codes.InvalidArgument error.
//   - If above the highest indexed height, return codes.InvalidArgument error.
//
// 5. If validation passes, return the adjusted start height.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if the start height is out of bounds based on indexed heights.
// - codes.FailedPrecondition - if the index reporter is not ready yet.
// - codes.Internal - for any other error during validation.
func (b *BaseTrackerImpl) checkStartHeight(height uint64) (uint64, error) {
	// if the start block is the root block, there will not be an execution data. skip it and
	// begin from the next block.
	if height == b.rootBlockHeight {
		height = b.rootBlockHeight + 1
	}

	if !b.useIndex {
		return height, nil
	}

	lowestHeight, highestHeight, err := b.getIndexedHeightBound()
	if err != nil {
		return 0, err
	}

	if height < lowestHeight {
		return 0, status.Errorf(codes.InvalidArgument, "start height %d is lower than lowest indexed height %d", height, lowestHeight)
	}

	if height > highestHeight {
		return 0, status.Errorf(codes.InvalidArgument, "start height %d is higher than highest indexed height %d", height, highestHeight)
	}

	return height, nil
}

// getIndexedHeightBound returns the lowest and highest indexed block heights
// Expected errors during normal operation:
// - codes.FailedPrecondition: if the index reporter is not ready yet.
// - codes.Internal: if there was any other error getting the heights.
func (b *BaseTrackerImpl) getIndexedHeightBound() (uint64, uint64, error) {
	lowestHeight, err := b.indexReporter.LowestIndexedHeight()
	if err != nil {
		if errors.Is(err, storage.ErrHeightNotIndexed) || errors.Is(err, indexer.ErrIndexNotInitialized) {
			// the index is not ready yet, but likely will be eventually
			return 0, 0, status.Errorf(codes.FailedPrecondition, "failed to get lowest indexed height: %v", err)
		}
		return 0, 0, rpc.ConvertError(err, "failed to get lowest indexed height", codes.Internal)
	}

	highestHeight, err := b.indexReporter.HighestIndexedHeight()
	if err != nil {
		if errors.Is(err, storage.ErrHeightNotIndexed) || errors.Is(err, indexer.ErrIndexNotInitialized) {
			// the index is not ready yet, but likely will be eventually
			return 0, 0, status.Errorf(codes.FailedPrecondition, "failed to get highest indexed height: %v", err)
		}
		return 0, 0, rpc.ConvertError(err, "failed to get highest indexed height", codes.Internal)
	}

	return lowestHeight, highestHeight, nil
}
