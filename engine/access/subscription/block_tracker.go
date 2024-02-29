package subscription

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/state/protocol"

	"github.com/onflow/flow-go/storage"
)

// BlockTracker is an interface for tracking blocks and handling block-related operations.
type BlockTracker interface {
	BaseTracker
	// GetHighestHeight returns the highest height based on the specified block status.
	//
	// Expected errors during normal operation:
	// - codes.InvalidArgument: If the specified block status is unsupported.
	GetHighestHeight(flow.BlockStatus) (uint64, error)
	// ProcessOnFinalizedBlock drives the subscription logic when a block is finalized.
	// The input to this callback is treated as trusted. This method should be executed on
	// `OnFinalizedBlock` notifications from the node-internal consensus instance.
	ProcessOnFinalizedBlock() error
}

var _ BlockTracker = (*BlockTrackerImpl)(nil)

// BlockTrackerImpl is an implementation of the BlockTracker interface.
type BlockTrackerImpl struct {
	*BaseTrackerImpl
	state       protocol.State
	broadcaster *engine.Broadcaster

	// finalizedHighestHeight contains the highest consecutive block height for which we have received a new notification.
	finalizedHighestHeight counters.StrictMonotonousCounter
	// sealedHighestHeight contains the highest consecutive block height for which we have received a new notification.
	sealedHighestHeight counters.StrictMonotonousCounter
}

// NewBlockTracker creates a new BlockTrackerImpl instance.
//
// Parameters:
// - state: The protocol state used for retrieving block information.
// - rootHeight: The root block height, serving as the baseline for calculating the start height.
// - headers: The storage headers for accessing block headers.
// - broadcaster: The engine broadcaster for publishing notifications.
// - indexReporter: The index reporter for checking indexed block heights.
// - useIndex: A flag indicating whether to use indexed block heights for validation.
//
// Returns:
// - *BlockTrackerImpl: A new instance of BlockTrackerImpl.
// - error: An error indicating the result of the operation, if any.
func NewBlockTracker(
	state protocol.State,
	rootHeight uint64,
	headers storage.Headers,
	broadcaster *engine.Broadcaster,
	indexReporter state_synchronization.IndexReporter,
	useIndex bool,
) (*BlockTrackerImpl, error) {
	lastFinalized, err := state.Final().Head()
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not retrieve last finalized block: %w", err)
	}

	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not retrieve last sealed block: %w", err)
	}

	return &BlockTrackerImpl{
		BaseTrackerImpl:        NewBaseTrackerImpl(rootHeight, state, headers, indexReporter, useIndex),
		state:                  state,
		finalizedHighestHeight: counters.NewMonotonousCounter(lastFinalized.Height),
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
// Expected errors during normal operation:
// - codes.InvalidArgument: If both startBlockID and startHeight are provided.
// - storage.ErrNotFound`: If a block is provided and does not exist.
// - codes.Internal: If there is an internal error.
func (b *BlockTrackerImpl) GetStartHeight(ctx context.Context, startBlockID flow.Identifier, startHeight uint64) (height uint64, err error) {
	return b.BaseTrackerImpl.GetStartHeight(ctx, startBlockID, startHeight)
}

// GetHighestHeight returns the highest height based on the specified block status.
//
// Returns expected errors:
// - codes.InvalidArgument: If the specified block status is unsupported.
func (b *BlockTrackerImpl) GetHighestHeight(blockStatus flow.BlockStatus) (uint64, error) {
	switch blockStatus {
	case flow.BlockStatusFinalized:
		return b.finalizedHighestHeight.Value(), nil
	case flow.BlockStatusSealed:
		return b.sealedHighestHeight.Value(), nil
	default:
		return 0, status.Errorf(codes.InvalidArgument, "could not get highest height for unsupported status")
	}
}

// ProcessOnFinalizedBlock drives the subscription logic when a block is finalized.
// The input to this callback is treated as trusted. This method should be executed on
// `OnFinalizedBlock` notifications from the node-internal consensus instance.
func (b *BlockTrackerImpl) ProcessOnFinalizedBlock() error {
	// get the finalized header from state
	finalizedHeader, err := b.state.Final().Head()
	if err != nil {
		return fmt.Errorf("unable to get latest finalized header: %w", err)
	}

	if ok := b.finalizedHighestHeight.Set(finalizedHeader.Height); !ok {
		return nil
	}

	// get the latest seal header from storage
	sealedHeader, err := b.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("unable to get latest sealed header: %w", err)
	}

	if ok := b.sealedHighestHeight.Set(sealedHeader.Height); !ok {
		return nil
	}

	b.broadcaster.Publish()

	return nil
}
