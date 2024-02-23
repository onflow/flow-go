package subscription

import (
	"context"
	"fmt"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// GetStartHeightFunc is a function type for getting the start height.
type GetStartHeightFunc func(context.Context, flow.Identifier, uint64, flow.BlockStatus) (uint64, error)

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

// ChainStateTracker represents state tracker for new blocks (finalized and sealed) and handles block-related operations.
type ChainStateTracker interface {
	// GetStartHeight returns the start height to use when searching.
	// Only one of startBlockID and startHeight may be set. Otherwise, an InvalidArgument error is returned.
	// If a block is provided and does not exist, a NotFound error is returned.
	// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
	GetStartHeight(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, blockStatus flow.BlockStatus) (uint64, error)

	// GetHighestHeight returns the highest height based on the specified block status.
	// Only flow.BlockStatusFinalized and flow.BlockStatusSealed are allowed.
	GetHighestHeight(blockStatus flow.BlockStatus) (uint64, error)

	// ProcessOnFinalizedBlock drives the subscription logic when a block is finalized.
	// The input to this callback is treated as trusted. This method should be executed on
	// `OnFinalizedBlock` notifications from the node-internal consensus instance.
	// No errors expected during normal operations.
	ProcessOnFinalizedBlock() error
}

var _ ChainStateTracker = (*ChainStateTrackerImpl)(nil)

// ChainStateTrackerImpl tracks for new blocks and handles block-related operations.
type ChainStateTrackerImpl struct {
	state           protocol.State
	headers         storage.Headers
	broadcaster     *engine.Broadcaster
	indexReporter   state_synchronization.IndexReporter
	useIndex        bool
	rootBlockHeight uint64

	// finalizedHighestHeight contains the highest consecutive block height for which we have received a new notification.
	finalizedHighestHeight counters.StrictMonotonousCounter
	// sealedHighestHeight contains the highest consecutive block height for which we have received a new notification.
	sealedHighestHeight counters.StrictMonotonousCounter
}

// NewChainStateTracker creates a new ChainStateTrackerImpl instance.
func NewChainStateTracker(
	state protocol.State,
	rootHeight uint64,
	headers storage.Headers,
	highestAvailableFinalizedHeight uint64,
	broadcaster *engine.Broadcaster,
	indexReporter state_synchronization.IndexReporter,
	useIndex bool,
) (*ChainStateTrackerImpl, error) {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve last sealed block: %w", err)
	}

	return &ChainStateTrackerImpl{
		state:                  state,
		rootBlockHeight:        rootHeight,
		headers:                headers,
		finalizedHighestHeight: counters.NewMonotonousCounter(highestAvailableFinalizedHeight),
		sealedHighestHeight:    counters.NewMonotonousCounter(lastSealed.Height),
		broadcaster:            broadcaster,
		indexReporter:          indexReporter,
		useIndex:               useIndex,
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
// Errors:
// - codes.InvalidArgument: If blockStatus is flow.BlockStatusUnknown, or both startBlockID and startHeight are provided.
// - storage.ErrNotFound`: If a block is provided and does not exist.
// - codes.Internal: If there is an internal error.
func (h *ChainStateTrackerImpl) GetStartHeight(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, blockStatus flow.BlockStatus) (height uint64, err error) {
	// block status could be only sealed and finalized
	if blockStatus == flow.BlockStatusUnknown {
		return 0, status.Errorf(codes.InvalidArgument, "block status could not be unknown")
	}

	// make sure only one of start block ID and start height is provided
	if startBlockID != flow.ZeroID && startHeight > 0 {
		return 0, status.Errorf(codes.InvalidArgument, "only one of start block ID and start height may be provided")
	}

	// ensure that the resolved start height is available
	defer func() {
		if err == nil {
			height, err = h.checkStartHeight(height)
		}
	}()

	if startBlockID != flow.ZeroID {
		return h.startHeightFromBlockID(startBlockID)
	}

	if startHeight > 0 {
		return h.startHeightFromHeight(startHeight)
	}

	// if no start block was provided, use the latest sealed block
	header, err := h.state.Sealed().Head()
	if err != nil {
		// In the RPC engine, if we encounter an error from the protocol state indicating state corruption,
		// we should halt processing requests, but do throw an exception which might cause a crash:
		// - It is unsafe to process requests if we have an internally bad State.
		// - We would like to avoid throwing an exception as a result of an Access API request by policy
		//   because this can cause DOS potential
		// - Since the protocol state is widely shared, we assume that in practice another component will
		//   observe the protocol state error and throw an exception.
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return 0, err
	}

	return header.Height, nil
}

func (h *ChainStateTrackerImpl) startHeightFromBlockID(startBlockID flow.Identifier) (uint64, error) {
	header, err := h.headers.ByBlockID(startBlockID)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for block %v: %w", startBlockID, err))
	}
	return header.Height, nil
}

func (h *ChainStateTrackerImpl) startHeightFromHeight(startHeight uint64) (uint64, error) {
	if startHeight < h.rootBlockHeight {
		return 0, status.Errorf(codes.InvalidArgument, "start height must be greater than or equal to the root height %d", h.rootBlockHeight)
	}

	header, err := h.headers.ByHeight(startHeight)
	if err != nil {
		return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for height %d: %w", startHeight, err))
	}
	return header.Height, nil
}

func (h *ChainStateTrackerImpl) checkStartHeight(height uint64) (uint64, error) {
	// if the start block is the root block, there will not be an execution data. skip it and
	// begin from the next block.
	if height == h.rootBlockHeight {
		height = h.rootBlockHeight + 1
	}

	if !h.useIndex {
		return height, nil
	}

	lowestHeight, highestHeight, err := h.getIndexerHeights()
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

// getIndexerHeights returns the lowest and highest indexed block heights
// Expected errors during normal operation:
// - codes.FailedPrecondition: if the index reporter is not ready yet.
// - codes.Internal: if there was any other error getting the heights.
func (h *ChainStateTrackerImpl) getIndexerHeights() (uint64, uint64, error) {
	lowestHeight, err := h.indexReporter.LowestIndexedHeight()
	if err != nil {
		if errors.Is(err, storage.ErrHeightNotIndexed) || errors.Is(err, indexer.ErrIndexNotInitialized) {
			// the index is not ready yet, but likely will be eventually
			return 0, 0, status.Errorf(codes.FailedPrecondition, "failed to get lowest indexed height: %v", err)
		}
		return 0, 0, rpc.ConvertError(err, "failed to get lowest indexed height", codes.Internal)
	}

	highestHeight, err := h.indexReporter.HighestIndexedHeight()
	if err != nil {
		if errors.Is(err, storage.ErrHeightNotIndexed) || errors.Is(err, indexer.ErrIndexNotInitialized) {
			// the index is not ready yet, but likely will be eventually
			return 0, 0, status.Errorf(codes.FailedPrecondition, "failed to get highest indexed height: %v", err)
		}
		return 0, 0, rpc.ConvertError(err, "failed to get highest indexed height", codes.Internal)
	}

	return lowestHeight, highestHeight, nil
}

// GetHighestHeight returns the highest height based on the specified block status. Only flow.BlockStatusFinalized and flow.BlockStatusSealed allowed.
func (h *ChainStateTrackerImpl) GetHighestHeight(blockStatus flow.BlockStatus) (uint64, error) {
	switch blockStatus {
	case flow.BlockStatusFinalized:
		return h.finalizedHighestHeight.Value(), nil
	case flow.BlockStatusSealed:
		return h.sealedHighestHeight.Value(), nil
	default:
		return 0, status.Errorf(codes.InvalidArgument, "could not get highest height for unsupported status")
	}
}

// ProcessOnFinalizedBlock drives the subscription logic when a block is finalized.
// The input to this callback is treated as trusted. This method should be executed on
// `OnFinalizedBlock` notifications from the node-internal consensus instance.
// No errors expected during normal operations.
func (h *ChainStateTrackerImpl) ProcessOnFinalizedBlock() error {
	// get the finalized header from state
	finalizedHeader, err := h.state.Final().Head()
	if err != nil {
		return fmt.Errorf("unable to get latest finalized header: %w", err)
	}

	if ok := h.finalizedHighestHeight.Set(finalizedHeader.Height); !ok {
		return nil
	}

	// get the latest seal header from storage
	sealedHeader, err := h.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("unable to get latest sealed header: %w", err)
	}

	if ok := h.sealedHighestHeight.Set(sealedHeader.Height); !ok {
		return nil
	}

	h.broadcaster.Publish()

	return nil
}
