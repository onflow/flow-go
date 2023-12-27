package subscription

import (
	"fmt"
	"sync/atomic"
	"time"

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

type StreamingData struct {
	MaxStreams  int32
	StreamCount atomic.Int32
}

type Config struct {
	Broadcaster            *engine.Broadcaster
	SendTimeout            time.Duration
	ResponseLimit          float64
	SendBufferSize         int
	RootHeight             uint64
	HighestAvailableHeight uint64
	Seals                  storage.Seals
}

type SubscriptionHandler struct {
	log         zerolog.Logger
	state       protocol.State
	headers     storage.Headers
	seals       storage.Seals
	broadcaster *engine.Broadcaster
	RootHeight  uint64
	RootBlockID flow.Identifier

	// finalizedHighestHeight contains the highest consecutive block height for which we have received a
	// new Execution Data notification.
	finalizedHighestHeight counters.StrictMonotonousCounter

	sealedHighestHeight counters.StrictMonotonousCounter // monotonous counter for last sealed block height
}

func NewSubscriptionBackendHandler(
	log zerolog.Logger,
	state protocol.State,
	rootHeight uint64,
	headers storage.Headers,
	highestAvailableFinalizedHeight uint64,
	seals storage.Seals,
	broadcaster *engine.Broadcaster,
) (*SubscriptionHandler, error) {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve last sealed block: %w", err)
	}

	return &SubscriptionHandler{
		log:                    log,
		state:                  state,
		RootHeight:             rootHeight,
		RootBlockID:            flow.ZeroID,
		headers:                headers,
		finalizedHighestHeight: counters.NewMonotonousCounter(highestAvailableFinalizedHeight),
		sealedHighestHeight:    counters.NewMonotonousCounter(lastSealed.Height),
		seals:                  seals,
		broadcaster:            broadcaster,
	}, nil
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

// GetStartHeightSealed returns the start height to use when searching.
// Only one of startBlockID and startHeight may be set. Otherwise, an InvalidArgument error is returned.
// If a block is provided and does not exist, a NotFound error is returned.
// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
func (h *SubscriptionHandler) GetStartHeightSealed(startBlockID flow.Identifier, startHeight uint64) (uint64, error) {
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
	}

	lastSealed, err := h.state.Sealed().Head()
	if err != nil {
		return 0, status.Errorf(codes.Internal, "could not get latest sealed block: %v", err)
	}

	if header != nil {
		if header.Height > lastSealed.Height {
			return 0, status.Errorf(codes.InvalidArgument, "provided start block must be sealed. Latest sealed block %V with the height %d", lastSealed.ID(), lastSealed.Height)
		}

		return header.Height, nil
	}

	// if no start block was provided, use the latest sealed block
	return lastSealed.Height, nil
}

// SetFinalizedHighestHeight sets the highest finalized block height.
func (h *SubscriptionHandler) SetFinalizedHighestHeight(height uint64) bool {
	return h.finalizedHighestHeight.Set(height)
}

func (h *SubscriptionHandler) GetFinalizedHighestHeight() uint64 {
	return h.finalizedHighestHeight.Value()
}

// SetSealedHighestHeight sets the highest sealed block height.
func (h *SubscriptionHandler) SetSealedHighestHeight(height uint64) bool {
	return h.sealedHighestHeight.Set(height)
}

func (h *SubscriptionHandler) GetSealedHighestHeight() uint64 {
	return h.sealedHighestHeight.Value()
}

func (h *SubscriptionHandler) ProcessSubscriptionOnFinalizedBlock(finalizedHeader *flow.Header) error {
	if ok := h.SetFinalizedHighestHeight(finalizedHeader.Height); !ok {
		h.log.Debug().Msg("finalized block already received")
		return nil
	}

	// retrieve latest _finalized_ seal in the fork with head finalizedBlock and update last
	// sealed height; we do _not_ bail, because we want to re-request approvals
	// especially, when sealing is stuck, i.e. last sealed height does not increase
	finalizedSeal, err := h.seals.HighestInFork(finalizedHeader.ID())
	if err != nil {
		return fmt.Errorf("could not retrieve finalizedSeal for finalized block %s", finalizedHeader.ID())
	}
	lastBlockWithFinalizedSeal, err := h.headers.ByBlockID(finalizedSeal.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve last sealed block %v: %w", finalizedSeal.BlockID, err)
	}

	if ok := h.SetSealedHighestHeight(lastBlockWithFinalizedSeal.Height); !ok {
		h.log.Debug().Msg("sealed block already received")
	}

	h.broadcaster.Publish()

	return nil
}
