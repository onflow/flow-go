package subscription

import (
	"fmt"

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
	// GetHighestHeight returns the highest height based on the specified block status which could be only BlockStatusSealed or BlockStatusFinalized.
	// No errors are expected during normal operation.
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
//
// Returns:
// - *BlockTrackerImpl: A new instance of BlockTrackerImpl.
// - error: An error indicating the result of the operation, if any.
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
		BaseTrackerImpl:        NewBaseTrackerImpl(rootHeight, state, headers),
		state:                  state,
		finalizedHighestHeight: counters.NewMonotonousCounter(lastFinalized.Height),
		sealedHighestHeight:    counters.NewMonotonousCounter(lastSealed.Height),
		broadcaster:            broadcaster,
	}, nil
}

// GetHighestHeight returns the highest height based on the specified block status.
//
// Parameters:
// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
//
// No errors are expected during normal operation.
func (b *BlockTrackerImpl) GetHighestHeight(blockStatus flow.BlockStatus) (uint64, error) {
	switch blockStatus {
	case flow.BlockStatusFinalized:
		return b.finalizedHighestHeight.Value(), nil
	case flow.BlockStatusSealed:
		return b.sealedHighestHeight.Value(), nil
	}
	return 0, fmt.Errorf("invalid block status: %s", blockStatus)
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

	if !b.finalizedHighestHeight.Set(finalizedHeader.Height) {
		return nil
	}

	// get the latest seal header from state
	sealedHeader, err := b.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("unable to get latest sealed header: %w", err)
	}

	if !b.sealedHighestHeight.Set(sealedHeader.Height) {
		return nil
	}

	b.broadcaster.Publish()

	return nil
}
