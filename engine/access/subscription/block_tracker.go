package subscription

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// GetHighestHeight is a function type for getting the highest height.
// Block status could be only BlockStatusSealed or BlockStatusFinalized.
type GetHighestHeight func(flow.BlockStatus) (uint64, error)

// BlockTracker is an interface for tracking blocks and handling block-related operations.
type BlockTracker interface {
	BaseTracker
	// GetHighestHeight returns the highest height based on the specified block status which could be only BlockStatusSealed
	// or BlockStatusFinalized.
	// No errors are expected during normal operation.
	GetHighestHeight(flow.BlockStatus) (uint64, error)
	// ProcessOnFinalizedBlock drives the subscription logic when a block is finalized.
	// The input to this callback is treated as trusted. This method should be executed on
	// `OnFinalizedBlock` notifications from the node-internal consensus instance.
	// No errors are expected during normal operation.
	ProcessOnFinalizedBlock() error
}

var _ BlockTracker = (*BlockTrackerImpl)(nil)

// BlockTrackerImpl is an implementation of the BlockTracker interface.
type BlockTrackerImpl struct {
	BaseTracker
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
//
// No errors are expected during normal operation.
func NewBlockTracker(
	state protocol.State,
	rootHeight uint64,
	headers storage.Headers,
	broadcaster *engine.Broadcaster,
) (*BlockTrackerImpl, error) {
	lastFinalized, err := state.Final().Head()
	if err != nil {
		// this header MUST exist in the db, otherwise the node likely has inconsistent state.
		return nil, irrecoverable.NewExceptionf("could not retrieve last finalized block: %w", err)
	}

	lastSealed, err := state.Sealed().Head()
	if err != nil {
		// this header MUST exist in the db, otherwise the node likely has inconsistent state.
		return nil, irrecoverable.NewExceptionf("could not retrieve last sealed block: %w", err)
	}

	return &BlockTrackerImpl{
		BaseTracker:            NewBaseTrackerImpl(rootHeight, state, headers),
		state:                  state,
		finalizedHighestHeight: counters.NewMonotonousCounter(lastFinalized.Height),
		sealedHighestHeight:    counters.NewMonotonousCounter(lastSealed.Height),
		broadcaster:            broadcaster,
	}, nil
}

// GetHighestHeight returns the highest height based on the specified block status.
//
// Parameters:
// - blockStatus: The status of the block. It is expected that blockStatus has already been handled for invalid flow.BlockStatusUnknown.
//
// Expected errors during normal operation:
// - codes.InvalidArgument    - if block status is flow.BlockStatusUnknown.
func (b *BlockTrackerImpl) GetHighestHeight(blockStatus flow.BlockStatus) (uint64, error) {
	switch blockStatus {
	case flow.BlockStatusFinalized:
		return b.finalizedHighestHeight.Value(), nil
	case flow.BlockStatusSealed:
		return b.sealedHighestHeight.Value(), nil
	}
	return 0, status.Errorf(codes.InvalidArgument, "invalid block status: %s", blockStatus)
}

// ProcessOnFinalizedBlock drives the subscription logic when a block is finalized.
// The input to this callback is treated as trusted. This method should be executed on
// `OnFinalizedBlock` notifications from the node-internal consensus instance.
// No errors are expected during normal operation. Any errors encountered should be
// treated as an exception.
func (b *BlockTrackerImpl) ProcessOnFinalizedBlock() error {
	// get the finalized header from state
	finalizedHeader, err := b.state.Final().Head()
	if err != nil {
		return irrecoverable.NewExceptionf("unable to get latest finalized header: %w", err)
	}

	if !b.finalizedHighestHeight.Set(finalizedHeader.Height) {
		return nil
	}

	// get the latest seal header from state
	sealedHeader, err := b.state.Sealed().Head()
	if err != nil {
		return irrecoverable.NewExceptionf("unable to get latest sealed header: %w", err)
	}

	_ = b.sealedHighestHeight.Set(sealedHeader.Height)
	// always publish since there is also a new finalized block.
	b.broadcaster.Publish()

	return nil
}
