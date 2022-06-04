package jobqueue

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// FinalizedBlockReader provides an abstraction for consumers to read blocks as job.
type FinalizedBlockReader struct {
	state  protocol.State
	blocks storage.Blocks
}

// NewFinalizedBlockReader creates and returns a FinalizedBlockReader.
func NewFinalizedBlockReader(state protocol.State, blocks storage.Blocks) *FinalizedBlockReader {
	return &FinalizedBlockReader{
		state:  state,
		blocks: blocks,
	}
}

// AtIndex returns the block job at the given index.
// The block job at an index is just the finalized block at that index (i.e., height).
func (r FinalizedBlockReader) AtIndex(index uint64) (module.Job, error) {
	block, err := r.blockByHeight(index)
	if err != nil {
		return nil, fmt.Errorf("could not get block by index %v: %w", index, err)
	}
	return BlockToJob(block), nil
}

// blockByHeight returns the block at the given height.
func (r FinalizedBlockReader) blockByHeight(height uint64) (*flow.Block, error) {
	block, err := r.blocks.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get block by height %d: %w", height, err)
	}

	return block, nil
}

// Head returns the last finalized height as job index.
func (r FinalizedBlockReader) Head() (uint64, error) {
	header, err := r.state.Final().Head()
	if err != nil {
		return 0, fmt.Errorf("could not get header of last finalized block: %w", err)
	}

	return header.Height, nil
}
