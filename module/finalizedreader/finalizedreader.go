package finalizedreader

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
)

type FinalizedReader struct {
	events.Noop // all unimplemented event consumers are no-ops
	lastHeight  *atomic.Uint64
	headers     storage.Headers
}

var _ protocol.Consumer = (*FinalizedReader)(nil)

func NewFinalizedReader(headers storage.Headers, lastHeight uint64) *FinalizedReader {
	return &FinalizedReader{
		lastHeight: atomic.NewUint64(lastHeight),
		headers:    headers,
	}
}

// FinalizedBlockIDAtHeight returns the block ID of the finalized block at the given height.
// It returns storage.NotFound if the given height has not been finalized yet
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

// BlockFinalized implements the protocol.Consumer interface, which allows FinalizedReader
// to consume finalized blocks from the protocol
func (r *FinalizedReader) BlockFinalized(h *flow.Header) {
	r.lastHeight.Store(h.Height)
}
