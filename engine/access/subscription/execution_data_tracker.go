package subscription

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
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
	headers storage.Headers

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
		BaseTrackerImpl: NewBaseTrackerImpl(rootHeight, state, headers, indexReporter, useIndex),
		headers:         headers,
		highestHeight:   counters.NewMonotonousCounter(highestAvailableFinalizedHeight),
	}, nil
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
// Returns expected errors::
// - codes.InvalidArgument: If blockStatus is flow.BlockStatusUnknown, or both startBlockID and startHeight are provided.
// - storage.ErrNotFound`: If a block is provided and does not exist.
// - codes.Internal: If there is an internal error.
func (e *ExecutionDataTrackerImpl) GetStartHeight(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) (height uint64, err error) {
	return e.BaseTrackerImpl.GetStartHeight(ctx, startBlockID, startHeight)
}

// OnExecutionData is used to notify the tracker when a new execution data is received.
//
// Returns expected errors:
// - storage.ErrNotFound if no block header with the given ID exists
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
