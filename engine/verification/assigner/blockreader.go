package assigner

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// FinalizedBlockReader provides an abstraction for consumers to read blocks
// as job
type FinalizedBlockReader struct {
	state  protocol.State
	blocks storage.Blocks
}

// the job index would just be the finalized block height
func (r FinalizedBlockReader) AtIndex(index int64) (module.Job, error) {
	block, err := r.blockByHeight(uint64(index))
	if err != nil {
		return nil, fmt.Errorf("could not get block by index %v: %w", index, err)
	}
	return blockToJob(block), nil
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

func blockToJob(block *flow.Block) *BlockJob {
	return &BlockJob{Block: block}
}

// Head returns the last finalized height as job index
func (r *FinalizedBlockReader) Head() (int64, error) {
	return 0, fmt.Errorf("return the last finalized height")
}
