package blockconsumer

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

// newFinalizedBlockReader creates and returns a FinalizedBlockReader.
func newFinalizedBlockReader(state protocol.State, blocks storage.Blocks) *FinalizedBlockReader {
	return &FinalizedBlockReader{
		state:  state,
		blocks: blocks,
	}
}

// AtIndex returns the block job at the given index.
// The block job at an index is just the finalized block at that index (i.e., height).
func (r FinalizedBlockReader) AtIndex(index int64) (module.Job, error) {
	block, err := r.blockByHeight(uint64(index))
	if err != nil {
		return nil, fmt.Errorf("could not get block by index %v: %w", index, err)
	}
	return toJob(block), nil
}

// blockByHeight returns the block at the given height.
func (r FinalizedBlockReader) blockByHeight(height uint64) (*flow.Block, error) {
	header, err := r.state.AtHeight(height).Head()
	if err != nil {
		return nil, fmt.Errorf("could not get header by height %v: %w", height, err)
	}

	blockID := header.ID()
	block, err := r.blocks.ByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get block by ID %v: %w", blockID, err)
	}

	return block, nil
}

// Head returns the last finalized height as job index.
func (r FinalizedBlockReader) Head() (int64, error) {
	header, err := r.state.Final().Head()
	if err != nil {
		return 0, fmt.Errorf("could not get header of last finalized block: %w", err)
	}

	return int64(header.Height), nil
}

// toJob converts the block to a BlockJob.
func toJob(block *flow.Block) *BlockJob {
	return &BlockJob{Block: block}
}
