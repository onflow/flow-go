package subscription

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ExecutionDataTracker is an interface for tracking the highest consecutive block height for which we have received a
// new Execution Data notification
type ExecutionDataTracker interface {
	BaseTracker
	// GetHighestHeight returns the highest height that we have consecutive execution data for.
	GetHighestHeight() uint64
	// OnExecutionData is used to notify the tracker when a new execution data is received.
	// No errors expected during normal operations.
	OnExecutionData(*execution_data.BlockExecutionDataEntity) error
}

var _ ExecutionDataTracker = (*ExecutionDataTrackerImpl)(nil)

// ExecutionDataTrackerImpl is an implementation of the ExecutionDataTracker interface.
type ExecutionDataTrackerImpl struct {
	*BaseTrackerImpl
	headers       storage.Headers
	indexReporter state_synchronization.IndexReporter
	useIndex      bool

	// highestHeight contains the highest consecutive block height that we have consecutive execution data for
	highestHeight counters.StrictMonotonousCounter
}

// NewExecutionDataTracker creates a new ExecutionDataTrackerImpl instance.
//
// Parameters:
// - state: The protocol state used for retrieving block information.
// - rootHeight: The root block height, serving as the baseline for calculating the start height.
// - headers: The storage headers for accessing block headers.
// - highestAvailableFinalizedHeight: The highest available finalized block height.
// - indexReporter: The index reporter for checking indexed block heights.
// - useIndex: A flag indicating whether to use indexed block heights for validation.
//
// Returns:
// - *ExecutionDataTrackerImpl: A new instance of ExecutionDataTrackerImpl.
// - error: An error indicating the result of the operation, if any.
func NewExecutionDataTracker(
	state protocol.State,
	rootHeight uint64,
	headers storage.Headers,
	highestAvailableFinalizedHeight uint64,
	indexReporter state_synchronization.IndexReporter,
	useIndex bool,
) (*ExecutionDataTrackerImpl, error) {
	return &ExecutionDataTrackerImpl{
		BaseTrackerImpl: NewBaseTrackerImpl(rootHeight, state, headers),
		headers:         headers,
		highestHeight:   counters.NewMonotonousCounter(highestAvailableFinalizedHeight),
		indexReporter:   indexReporter,
		useIndex:        useIndex,
	}, nil
}

// GetStartHeight returns the start height to use when searching.
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
// - codes.InvalidArgument - if both startBlockID and startHeight are provided, if the start height is less than the root block height,
// if the start height is out of bounds based on indexed heights (when index is used).
// - storage.ErrNotFound   - if a block is provided and does not exist.
// - codes.Internal        - if there is an internal error.
func (e *ExecutionDataTrackerImpl) GetStartHeight(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
	height, err := e.BaseTrackerImpl.GetStartHeight(ctx, startBlockID, startHeight)
	if err != nil {
		return 0, err
	}

	// ensure that the resolved start height is available
	height, err = e.checkStartHeight(height)
	if err != nil {
		return 0, err
	}

	return height, nil
}

// OnExecutionData is used to notify the tracker when a new execution data is received.
//
// Expected errors during normal operation:
// - storage.ErrNotFound - if no block header with the given ID exists
func (e *ExecutionDataTrackerImpl) OnExecutionData(executionData *execution_data.BlockExecutionDataEntity) error {
	header, err := e.headers.ByBlockID(executionData.BlockID)
	if err != nil {
		// if the execution data is available, the block must be locally finalized
		return err
	}

	// sets the highest height for which execution data is available.
	e.highestHeight.Set(header.Height)
	return nil
}

// GetHighestHeight returns the highest height that we have consecutive execution data for.
// No errors expected during normal operations.
func (e *ExecutionDataTrackerImpl) GetHighestHeight() uint64 {
	return e.highestHeight.Value()
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
// - codes.InvalidArgument    - if both startBlockID and startHeight are provided, if the start height is less than the
// root block height, if the start height is out of bounds based on indexed heights.
// - codes.FailedPrecondition - if the index reporter is not ready yet.
// - codes.Internal           - for any other error during validation.
func (e *ExecutionDataTrackerImpl) checkStartHeight(height uint64) (uint64, error) {
	// if the start block is the root block, there will not be an execution data. skip it and
	// begin from the next block.
	if height == e.rootBlockHeight {
		height = e.rootBlockHeight + 1
	}

	if !e.useIndex {
		return height, nil
	}

	lowestHeight, highestHeight, err := e.getIndexedHeightBound()
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
// - codes.FailedPrecondition - if the index reporter is not ready yet.
// - codes.Internal           - if there was any other error getting the heights.
func (e *ExecutionDataTrackerImpl) getIndexedHeightBound() (uint64, uint64, error) {
	lowestHeight, err := e.indexReporter.LowestIndexedHeight()
	if err != nil {
		if errors.Is(err, storage.ErrHeightNotIndexed) || errors.Is(err, indexer.ErrIndexNotInitialized) {
			// the index is not ready yet, but likely will be eventually
			return 0, 0, status.Errorf(codes.FailedPrecondition, "failed to get lowest indexed height: %v", err)
		}
		return 0, 0, rpc.ConvertError(err, "failed to get lowest indexed height", codes.Internal)
	}

	highestHeight, err := e.indexReporter.HighestIndexedHeight()
	if err != nil {
		if errors.Is(err, storage.ErrHeightNotIndexed) || errors.Is(err, indexer.ErrIndexNotInitialized) {
			// the index is not ready yet, but likely will be eventually
			return 0, 0, status.Errorf(codes.FailedPrecondition, "failed to get highest indexed height: %v", err)
		}
		return 0, 0, rpc.ConvertError(err, "failed to get highest indexed height", codes.Internal)
	}

	return lowestHeight, highestHeight, nil
}
