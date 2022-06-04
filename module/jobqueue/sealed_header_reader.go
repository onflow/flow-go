package jobqueue

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// SealedBlockHeaderReader provides an abstraction for consumers to read blocks as job.
type SealedBlockHeaderReader struct {
	state   protocol.State
	headers storage.Headers
}

// NewSealedBlockHeaderReader creates and returns a SealedBlockHeaderReader.
func NewSealedBlockHeaderReader(state protocol.State, headers storage.Headers) *SealedBlockHeaderReader {
	return &SealedBlockHeaderReader{
		state:   state,
		headers: headers,
	}
}

// AtIndex returns the block header job at the given index.
// The block header job at an index is just the finalized block header at that index (i.e., height).
func (r SealedBlockHeaderReader) AtIndex(index uint64) (module.Job, error) {
	header, err := r.blockByHeight(index)
	if err != nil {
		return nil, fmt.Errorf("could not get block by index %v: %w", index, err)
	}

	sealed, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get last sealed block height: %w", err)
	}

	if index > sealed {
		// return not found error to indicate there is no job available at this height
		return nil, fmt.Errorf("block at index %v is not sealed: %w", index, storage.ErrNotFound)
	}

	// the block at height index is sealed
	return BlockHeaderToJob(header), nil
}

// blockByHeight returns the block at the given height.
func (r SealedBlockHeaderReader) blockByHeight(height uint64) (*flow.Header, error) {
	block, err := r.headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get block by height %d: %w", height, err)
	}

	return block, nil
}

// Head returns the last sealed height as job index.
func (r SealedBlockHeaderReader) Head() (uint64, error) {
	header, err := r.state.Sealed().Head()
	if err != nil {
		return 0, fmt.Errorf("could not get header of last sealed block: %w", err)
	}

	return header.Height, nil
}
