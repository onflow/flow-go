package tracker

import (
	"context"

	"github.com/rs/zerolog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// maxIndexBlockDiff is the maximum difference between the highest indexed block height and the
	// provided start height to allow when starting a new stream.
	// this is used to account for small delays indexing or requests made to different ANs behind a
	// load balancer. The diff will result in the stream waiting a few blocks before starting.
	maxIndexBlockDiff = 30
)

// ExecutionDataTracker is an implementation of the ExecutionDataTracker interface.
type ExecutionDataTracker struct {
	*HeightTracker

	log           zerolog.Logger
	headers       storage.Headers
	broadcaster   *engine.Broadcaster
	indexReporter state_synchronization.IndexReporter
	useIndex      bool

	// highestHeight contains the highest consecutive block height that we have consecutive execution data for
	highestHeight counters.StrictMonotonicCounter
}

var _ subscription.ExecutionDataTracker = (*ExecutionDataTracker)(nil)

// NewExecutionDataTracker creates a new ExecutionDataTracker instance.
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
// - *ExecutionDataTracker: A new instance of ExecutionDataTracker.
func NewExecutionDataTracker(
	log zerolog.Logger,
	state protocol.State,
	rootHeight uint64,
	headers storage.Headers,
	broadcaster *engine.Broadcaster,
	highestAvailableFinalizedHeight uint64,
	indexReporter state_synchronization.IndexReporter,
	useIndex bool,
) *ExecutionDataTracker {
	return &ExecutionDataTracker{
		HeightTracker: NewHeightTracker(rootHeight, state, headers),
		log:           log,
		headers:       headers,
		broadcaster:   broadcaster,
		highestHeight: counters.NewMonotonicCounter(highestAvailableFinalizedHeight),
		indexReporter: indexReporter,
		useIndex:      useIndex,
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
// - codes.InvalidArgument    - if the start height is out of bounds based on indexed heights.
// - codes.FailedPrecondition - if the index reporter is not ready yet.
// - codes.Internal           - for any other error during validation.
func (e *ExecutionDataTracker) GetStartHeightFromBlockID(startBlockID flow.Identifier) (uint64, error) {
	// get start height based on the provided starting block id
	height, err := e.HeightTracker.GetStartHeightFromBlockID(startBlockID)
	if err != nil {
		return 0, err
	}

	// ensure that the resolved start height is available
	return e.checkStartHeight(height)
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
// - codes.InvalidArgument    - if the start height is less than the root block height, if the start height is out of bounds based on indexed heights
// - codes.NotFound           - if the header was not found in storage.
// - codes.FailedPrecondition - if the index reporter is not ready yet.
// - codes.Internal           - for any other error during validation.
func (e *ExecutionDataTracker) GetStartHeightFromHeight(startHeight uint64) (uint64, error) {
	// get start height based on the provided starting block height
	height, err := e.HeightTracker.GetStartHeightFromHeight(startHeight)
	if err != nil {
		return 0, err
	}

	// ensure that the resolved start height is available
	return e.checkStartHeight(height)
}

// GetStartHeightFromLatest returns the start height based on the latest sealed block.
//
// Parameters:
// - ctx: Context for the operation.
//
// Expected errors during normal operation:
// - codes.InvalidArgument    - if the start height is out of bounds based on indexed heights.
// - codes.FailedPrecondition - if the index reporter is not ready yet.
// - codes.Internal           - for any other error during validation.
func (e *ExecutionDataTracker) GetStartHeightFromLatest(ctx context.Context) (uint64, error) {
	// get start height based latest sealed block
	height, err := e.HeightTracker.GetStartHeightFromLatest(ctx)
	if err != nil {
		return 0, err
	}

	// ensure that the resolved start height is available
	return e.checkStartHeight(height)
}

// GetHighestHeight returns the highest height that we have consecutive execution data for.
func (e *ExecutionDataTracker) GetHighestHeight() uint64 {
	return e.highestHeight.Value()
}

// OnExecutionData is used to notify the tracker when new execution data is received.
func (e *ExecutionDataTracker) OnExecutionData(executionData *execution_data.BlockExecutionDataEntity) {
	log := e.log.With().Hex("block_id", logging.ID(executionData.BlockID)).Logger()
	log.Trace().Msg("received execution data")

	header, err := e.headers.ByBlockID(executionData.BlockID)
	if err != nil {
		// if the execution data is available, the block must be locally finalized
		log.Fatal().Err(err).Msg("failed to notify of new execution data")
		return
	}

	// sets the highest height for which execution data is available.
	_ = e.highestHeight.Set(header.Height)

	e.broadcaster.Publish()
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
// 1. If index usage is disabled, return the original height without further checks.
// 2. Retrieve the lowest and highest indexed block heights.
// 3. Check if the provided height is within the bounds of indexed heights.
//   - If below the lowest indexed height, return codes.InvalidArgument error.
//   - If above the highest indexed height, return codes.InvalidArgument error.
//
// 4. If validation passes, return the adjusted start height.
//
// Expected errors during normal operation:
// - codes.InvalidArgument    - if the start height is out of bounds based on indexed heights.
// - codes.FailedPrecondition - if the index reporter is not ready yet.
// - codes.Internal           - for any other error during validation.
func (e *ExecutionDataTracker) checkStartHeight(height uint64) (uint64, error) {
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

	// allow for a small difference between the highest indexed height and the provided height to
	// account for small delays indexing or requests made to different ANs behind a load balancer.
	// this will just result in the stream waiting a few blocks before starting.
	if height > highestHeight+maxIndexBlockDiff {
		return 0, status.Errorf(codes.InvalidArgument, "start height %d is higher than highest indexed height %d (maxIndexBlockDiff: %d)", height, highestHeight, maxIndexBlockDiff)
	}

	return height, nil
}

// getIndexedHeightBound returns the lowest and highest indexed block heights
// Expected errors during normal operation:
// - codes.FailedPrecondition - if the index reporter is not ready yet.
// - codes.Internal           - if there was any other error getting the heights.
func (e *ExecutionDataTracker) getIndexedHeightBound() (uint64, uint64, error) {
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
