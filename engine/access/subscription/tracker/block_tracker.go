package tracker

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// BlockTracker is an implementation of the BlockTracker interface.
type BlockTracker struct {
	*HeightTracker

	state       protocol.State
	broadcaster *engine.Broadcaster

	// finalizedHighestHeight contains the highest consecutive block height for which we have received a new notification.
	finalizedHighestHeight counters.StrictMonotonicCounter
	// sealedHighestHeight contains the highest consecutive block height for which we have received a new notification.
	sealedHighestHeight counters.StrictMonotonicCounter
}

var _ subscription.BlockTracker = (*BlockTracker)(nil)

// NewBlockTracker creates a new BlockTracker instance.
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
) (*BlockTracker, error) {
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

	return &BlockTracker{
		HeightTracker:          NewHeightTracker(rootHeight, state, headers),
		state:                  state,
		finalizedHighestHeight: counters.NewMonotonicCounter(lastFinalized.Height),
		sealedHighestHeight:    counters.NewMonotonicCounter(lastSealed.Height),
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
func (b *BlockTracker) GetHighestHeight(blockStatus flow.BlockStatus) (uint64, error) {
	switch blockStatus {
	case flow.BlockStatusFinalized:
		return b.finalizedHighestHeight.Value(), nil
	case flow.BlockStatusSealed:
		return b.sealedHighestHeight.Value(), nil
	default:
		return 0, status.Errorf(codes.InvalidArgument, "invalid block status: %s", blockStatus)
	}
}

// ProcessOnFinalizedBlock drives the subscription logic when a block is finalized.
// The input to this callback is treated as trusted. This method should be executed on
// `OnFinalizedBlock` notifications from the node-internal consensus instance.
// No errors are expected during normal operation. Any errors encountered should be
// treated as an exception.
func (b *BlockTracker) ProcessOnFinalizedBlock() error {
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
