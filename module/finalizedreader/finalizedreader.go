package finalizedreader

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type FinalizedReader struct {
	lastHeight *atomic.Uint64
	headers    storage.Headers
}

var _ protocol.Consumer = (*FinalizedReader)(nil)

func NewFinalizedReader(headers storage.Headers, lastHeight uint64) *FinalizedReader {
	return &FinalizedReader{
		lastHeight: atomic.NewUint64(lastHeight),
		headers:    headers,
	}
}

// FinalizedBlockIDAtHeight returns the block ID of the finalized block at the given height.
// It return storage.NotFound if the given height has not been finalized yet
// any other error returned are exceptions
func (r *FinalizedReader) FinalizedBlockIDAtHeight(height uint64) (flow.Identifier, error) {
	if height > r.lastHeight.Load() {
		return flow.ZeroID, fmt.Errorf("height not finalized (%v): %w", height, storage.ErrNotFound)
	}

	finalizedID, err := r.headers.BlockIDByHeight(height)
	if err != nil {
		return flow.ZeroID, err
	}

	return finalizedID, nil
}

func (r *FinalizedReader) LatestFinalizedHight() uint64 {
	return r.lastHeight.Load()
}

// BlockFinalized implements the protocol.Consumer interface, which allows FinalizedReader
// to consume finalized blocks from the protocol
func (r *FinalizedReader) BlockFinalized(h *flow.Header) {
	r.lastHeight.Store(h.Height)
}

func (r *FinalizedReader) BlockProcessable(h *flow.Header, qc *flow.QuorumCertificate) {
	// noop
}

func (r *FinalizedReader) EpochTransition(newEpochCounter uint64, first *flow.Header) {
	// noop
}

func (r *FinalizedReader) EpochSetupPhaseStarted(currentEpochCounter uint64, first *flow.Header) {
	// noop
}

func (r *FinalizedReader) EpochCommittedPhaseStarted(currentEpochCounter uint64, first *flow.Header) {
	// noop
}

func (r *FinalizedReader) EpochEmergencyFallbackTriggered() {
	// noop
}
