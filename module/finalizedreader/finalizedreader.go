package finalizedreader

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type FinalizedReader struct {
	protocol.Consumer
	lastHeight *atomic.Uint64
	headers    storage.Headers
}

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

	header, err := r.headers.ByHeight(height)
	if err != nil {
		return flow.ZeroID, err
	}

	return header.ID(), nil
}

// BlockFinalized implements the protocol.Consumer interface, which allows FinalizedReader
// to consume finalized blocks from the protocol
func (r *FinalizedReader) BlockFinalized(h *flow.Header) {
	r.lastHeight.Store(h.Height)
}
