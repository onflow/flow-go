package tracker

import (
	"context"

	"github.com/rs/zerolog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// ExecutionDataTracker is an interface for tracking the highest consecutive block height for which we have received a
// new Execution Data notification
type ExecutionDataTracker interface {
	BaseTracker

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
	// - codes.NotFound   - if a block is provided and does not exist.
	// - codes.Internal        - if there is an internal error.
	GetStartHeight(context.Context, flow.Identifier, uint64) (uint64, error)

	// GetHighestAvailableFinalizedHeight returns the highest finalized block height for which execution data is available.
	GetHighestAvailableFinalizedHeight() uint64

	// OnExecutionData is used to notify the tracker when a new execution data is received.
	OnExecutionData(*execution_data.BlockExecutionDataEntity)
}

var _ ExecutionDataTracker = (*ExecutionDataTrackerImpl)(nil)

// ExecutionDataTrackerImpl is an implementation of the ExecutionDataTracker interface.
type ExecutionDataTrackerImpl struct {
	*BaseTrackerImpl
	log         zerolog.Logger
	headers     storage.Headers
	broadcaster *engine.Broadcaster

	// contains the highest consecutive block height that we have consecutive execution data for
	highestAvailableFinalizedHeight counters.StrictMonotonicCounter
}

// NewExecutionDataTracker creates a new ExecutionDataTrackerImpl instance.
//
// Parameters:
// - log: The logger to use for logging.
// - state: The protocol state used for retrieving block information.
// - rootHeight: The root block height, serving as the baseline for calculating the start height.
// - headers: The storage headers for accessing block headers.
// - broadcaster: The engine broadcaster for publishing notifications.
// - highestAvailableFinalizedHeight: The highest available finalized block height.
// - indexReporter: The index reporter for checking indexed block heights.
// - useIndex: A flag indicating whether to use indexed block heights for validation.
//
// Returns:
// - *ExecutionDataTrackerImpl: A new instance of ExecutionDataTrackerImpl.
func NewExecutionDataTracker(
	log zerolog.Logger,
	state protocol.State,
	rootHeight uint64,
	headers storage.Headers,
	broadcaster *engine.Broadcaster,
	highestAvailableFinalizedHeight uint64,
) *ExecutionDataTrackerImpl {
	return &ExecutionDataTrackerImpl{
		BaseTrackerImpl:                 NewBaseTrackerImpl(rootHeight, state, headers),
		log:                             log,
		headers:                         headers,
		broadcaster:                     broadcaster,
		highestAvailableFinalizedHeight: counters.NewMonotonicCounter(highestAvailableFinalizedHeight),
	}
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
// - codes.NotFound        - if a block is provided and does not exist.
// - codes.Internal        - if there is an internal error.
func (e *ExecutionDataTrackerImpl) GetStartHeight(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
	if startBlockID != flow.ZeroID && startHeight > 0 {
		return 0, status.Errorf(codes.InvalidArgument, "only one of start block ID and start height may be provided")
	}

	// get the start height based on the provided starting block ID
	if startBlockID != flow.ZeroID {
		return e.GetStartHeightFromBlockID(startBlockID)
	}

	// get start height based on the provided starting block height
	if startHeight > 0 {
		return e.GetStartHeightFromHeight(startHeight)
	}

	return e.GetStartHeightFromLatest(ctx)
}

// GetHighestAvailableFinalizedHeight returns the highest finalized block height for which execution data is available.
func (e *ExecutionDataTrackerImpl) GetHighestAvailableFinalizedHeight() uint64 {
	return e.highestAvailableFinalizedHeight.Value()
}

// OnExecutionData is used to notify the underlying broadcaster when new execution data is received.
// Notification is not sent if the header storage hasn't received a block yet.
func (e *ExecutionDataTrackerImpl) OnExecutionData(executionData *execution_data.BlockExecutionDataEntity) {
	log := e.log.With().Hex("block_id", logging.ID(executionData.BlockID)).Logger()
	log.Trace().Msg("received execution data")

	// TODO: update this to throw an irrecoverable error
	header, err := e.headers.ByBlockID(executionData.BlockID)
	if err != nil {
		// if the execution data is available, the block must be locally finalized
		log.Fatal().Err(err).Msg("failed to notify broadcaster that new execution data is available")
		return
	}
	_ = e.highestAvailableFinalizedHeight.Set(header.Height)

	e.broadcaster.Publish()
}
