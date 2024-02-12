package subscription

import (
	"fmt"
	"sync/atomic"

	"github.com/rs/zerolog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// GetStartHeightFunc is a function type for getting the start height.
type GetStartHeightFunc func(flow.Identifier, uint64, flow.BlockStatus) (uint64, error)

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
	GetStartHeight(startBlockID flow.Identifier, startHeight uint64, blockStatus flow.BlockStatus) (uint64, error)

	// GetHighestHeight returns the highest height based on the specified block status. O
	// Only flow.BlockStatusFinalized and flow.BlockStatusSealed allowed.
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
	log         zerolog.Logger
	state       protocol.State
	headers     storage.Headers
	broadcaster *engine.Broadcaster
	RootHeight  uint64
	RootBlockID flow.Identifier

	// finalizedHighestHeight contains the highest consecutive block height for which we have received a new notification.
	finalizedHighestHeight counters.StrictMonotonousCounter
	// sealedHighestHeight contains the highest consecutive block height for which we have received a new notification.
	sealedHighestHeight counters.StrictMonotonousCounter
}

// NewChainStateTracker creates a new ChainStateTrackerImpl instance.
func NewChainStateTracker(
	log zerolog.Logger,
	state protocol.State,
	rootHeight uint64,
	headers storage.Headers,
	highestAvailableFinalizedHeight uint64,
	broadcaster *engine.Broadcaster,
) (*ChainStateTrackerImpl, error) {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve last sealed block: %w", err)
	}

	return &ChainStateTrackerImpl{
		log:                    log,
		state:                  state,
		RootHeight:             rootHeight,
		RootBlockID:            flow.ZeroID,
		headers:                headers,
		finalizedHighestHeight: counters.NewMonotonousCounter(highestAvailableFinalizedHeight),
		sealedHighestHeight:    counters.NewMonotonousCounter(lastSealed.Height),
		broadcaster:            broadcaster,
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
func (h *ChainStateTrackerImpl) GetStartHeight(startBlockID flow.Identifier, startHeight uint64, blockStatus flow.BlockStatus) (uint64, error) {
	// block status could be only sealed and finalized
	if blockStatus == flow.BlockStatusUnknown {
		return 0, status.Errorf(codes.InvalidArgument, "block status could not be unknown")
	}

	// make sure only one of start block ID and start height is provided
	if startBlockID != flow.ZeroID && startHeight > 0 {
		return 0, status.Errorf(codes.InvalidArgument, "only one of start block ID and start height may be provided")
	}

	if h.RootBlockID == flow.ZeroID {
		// cache the root block height and ID for runtime lookups.
		rootBlockID, err := h.headers.BlockIDByHeight(h.RootHeight)
		if err != nil {
			return 0, fmt.Errorf("could not get root block ID: %w", err)
		}
		h.RootBlockID = rootBlockID
	}

	// if the start block is the root block, there will not be an execution data. skip it and
	// begin from the next block.
	// Note: we can skip the block lookup since it was already done in the constructor
	if startBlockID == h.RootBlockID ||
		// Note: there is a corner case when rootBlockHeight == 0:
		// since the default value of an uint64 is 0, when checking if startHeight matches the root block
		// we also need to check that startBlockID is unset, otherwise we may incorrectly set the start height
		// for non-matching startBlockIDs.
		(startHeight == h.RootHeight && startBlockID == flow.ZeroID) {
		return h.RootHeight + 1, nil
	}

	var header *flow.Header
	var err error
	// invalid or missing block IDs will result in an error
	if startBlockID != flow.ZeroID {
		header, err = h.headers.ByBlockID(startBlockID)
		if err != nil {
			return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for block %v: %w", startBlockID, err))
		}

		if blockStatus == flow.BlockStatusFinalized {
			return header.Height, nil
		}
	}

	// heights that have not been indexed yet will result in an error
	if startHeight > 0 {
		if startHeight < h.RootHeight {
			return 0, status.Errorf(codes.InvalidArgument, "start height must be greater than or equal to the root height %d", h.RootHeight)
		}

		header, err = h.headers.ByHeight(startHeight)
		if err != nil {
			return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for height %d: %w", startHeight, err))
		}

		if blockStatus == flow.BlockStatusFinalized {
			return header.Height, nil
		}
	}

	lastSealed, err := h.state.Sealed().Head()
	if err != nil {
		return 0, status.Errorf(codes.Internal, "could not get latest sealed block: %v", err)
	}

	// this checking if start block is sealed
	if header != nil {
		if header.Height > lastSealed.Height {
			return 0, status.Errorf(codes.InvalidArgument, "provided start block must be sealed. Latest sealed block %v with the height %d", lastSealed.ID(), lastSealed.Height)
		}

		return header.Height, nil
	}

	// if no start block was provided, use the latest sealed block
	return lastSealed.Height, nil
}

// GetHighestHeight returns the highest height based on the specified block status. Only flow.BlockStatusFinalized and flow.BlockStatusSealed allowed.
func (h *ChainStateTrackerImpl) GetHighestHeight(blockStatus flow.BlockStatus) (uint64, error) {
	switch blockStatus {
	case flow.BlockStatusFinalized:
		return h.finalizedHighestHeight.Value(), nil
	case flow.BlockStatusSealed:
		return h.sealedHighestHeight.Value(), nil
	case flow.BlockStatusUnknown:
		return 0, status.Errorf(codes.InvalidArgument, "could not get highest height for block with unknown status")
	}

	return 0, status.Errorf(codes.InvalidArgument, "could not get highest height for invalid status")
}

// ProcessOnFinalizedBlock drives the subscription logic when a block is finalized.
// The input to this callback is treated as trusted. This method should be executed on
// `OnFinalizedBlock` notifications from the node-internal consensus instance.
// No errors expected during normal operations.
func (h *ChainStateTrackerImpl) ProcessOnFinalizedBlock() error {
	// get the finalized header from state
	finalizedHeader, err := h.state.Final().Head()
	if err != nil {
		// this header MUST exist in the db, otherwise the node likely has inconsistent state.
		// Don't crash as a result of an external API request, but other components will likely panic.
		h.log.Err(err).Msg("failed to get latest block header. potentially inconsistent protocol state.")
		return status.Errorf(codes.Internal, "unable to get latest finalized header: %v", err)
	}

	if ok := h.finalizedHighestHeight.Set(finalizedHeader.Height); !ok {
		return nil
	}

	// get the latest seal header from storage
	sealedHeader, err := h.state.Sealed().Head()
	if err != nil {
		// this header MUST exist in the db, otherwise the node likely has inconsistent state.
		// Don't crash as a result of an external API request, but other components will likely panic.
		h.log.Err(err).Msg("failed to get latest block header. potentially inconsistent protocol state.")
		return status.Errorf(codes.Internal, "unable to get latest sealed header: %v", err)
	}

	if ok := h.sealedHighestHeight.Set(sealedHeader.Height); !ok {
		return nil
	}

	h.broadcaster.Publish()

	return nil
}
