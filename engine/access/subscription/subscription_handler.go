package subscription

import (
	"fmt"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type StreamingData struct {
	MaxStreams  int32
	StreamCount atomic.Int32
}

type SubscriptionHandler struct {
	state       protocol.State
	headers     storage.Headers
	RootHeight  uint64
	RootBlockID flow.Identifier

	// highestHeight contains the highest consecutive block height for which we have received a
	// new Execution Data notification.
	highestHeight counters.StrictMonotonousCounter
}

func NewSubscriptionBackendHandler(state protocol.State, rootHeight uint64, headers storage.Headers, highestAvailableHeight uint64) *SubscriptionHandler {
	return &SubscriptionHandler{
		state:         state,
		RootHeight:    rootHeight,
		RootBlockID:   flow.ZeroID,
		headers:       headers,
		highestHeight: counters.NewMonotonousCounter(highestAvailableHeight),
	}
}

// GetStartHeight returns the start height to use when searching.
// Only one of startBlockID and startHeight may be set. Otherwise, an InvalidArgument error is returned.
// If a block is provided and does not exist, a NotFound error is returned.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
func (h *SubscriptionHandler) GetStartHeight(startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
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

	// invalid or missing block IDs will result in an error
	if startBlockID != flow.ZeroID {
		header, err := h.headers.ByBlockID(startBlockID)
		if err != nil {
			return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for block %v: %w", startBlockID, err))
		}
		return header.Height, nil
	}

	// heights that have not been indexed yet will result in an error
	if startHeight > 0 {
		if startHeight < h.RootHeight {
			return 0, status.Errorf(codes.InvalidArgument, "start height must be greater than or equal to the root height %d", h.RootHeight)
		}

		header, err := h.headers.ByHeight(startHeight)
		if err != nil {
			return 0, rpc.ConvertStorageError(fmt.Errorf("could not get header for height %d: %w", startHeight, err))
		}
		return header.Height, nil
	}

	// if no start block was provided, use the latest sealed block
	header, err := h.state.Sealed().Head()
	if err != nil {
		return 0, status.Errorf(codes.Internal, "could not get latest sealed block: %v", err)
	}
	return header.Height, nil
}

// SetHighestHeight sets the highest height for which execution data is available.
func (h *SubscriptionHandler) SetHighestHeight(height uint64) bool {
	return h.highestHeight.Set(height)
}

func (h *SubscriptionHandler) GetHighestHeight() uint64 {
	return h.highestHeight.Value()
}
