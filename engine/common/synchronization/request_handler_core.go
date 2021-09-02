package synchronization

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

type ConsensusRequestHandlerCore struct {
	log    zerolog.Logger
	blocks storage.Blocks
	core   module.SyncCore
}

func NewRequestHandlerCore(
	log zerolog.Logger,
	blocks storage.Blocks,
	core module.SyncCore,
) *ConsensusRequestHandlerCore {
	return &ConsensusRequestHandlerCore{
		log:    log,
		blocks: blocks,
		core:   core,
	}
}

func (c *ConsensusRequestHandlerCore) HandleSyncRequest(req *messages.SyncRequest, final *flow.Header) (interface{}, error) {
	// queue any missing heights as needed
	c.core.HandleHeight(final, req.Height)

	// don't bother sending a response if we're within tolerance or if we're
	// behind the requester
	if c.core.WithinTolerance(final, req.Height) || req.Height > final.Height {
		return nil, nil
	}

	// if we're sufficiently ahead of the requester, send a response
	return &messages.SyncResponse{
		Height: final.Height,
		Nonce:  req.Nonce,
	}, nil
}

func (c *ConsensusRequestHandlerCore) HandleRangeRequest(req *messages.RangeRequest, finalized *flow.Header) (interface{}, error) {
	// if we don't have anything to send, we can bail right away
	if finalized.Height < req.FromHeight || req.FromHeight > req.ToHeight {
		return nil, nil
	}

	// get all the blocks, one by one
	blocks := make([]*flow.Block, 0, req.ToHeight-req.FromHeight+1)
	for height := req.FromHeight; height <= req.ToHeight; height++ {
		block, err := c.blocks.ByHeight(height)
		if errors.Is(err, storage.ErrNotFound) {
			c.log.Error().Uint64("height", height).Msg("skipping unknown heights")
			break
		}
		if err != nil {
			return nil, fmt.Errorf("could not get block for height (%d): %w", height, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		c.log.Debug().Msg("skipping empty range response")
		return nil, nil
	}

	// send the response
	return &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}, nil
}

func (c *ConsensusRequestHandlerCore) HandleBatchRequest(req *messages.BatchRequest) (interface{}, error) {
	// deduplicate the block IDs in the batch request
	blockIDs := make(map[flow.Identifier]struct{})
	for _, blockID := range req.BlockIDs {
		blockIDs[blockID] = struct{}{}
	}

	// try to get all the blocks by ID
	blocks := make([]*flow.Block, 0, len(blockIDs))
	for blockID := range blockIDs {
		block, err := c.blocks.ByID(blockID)
		if errors.Is(err, storage.ErrNotFound) {
			c.log.Debug().Hex("block_id", blockID[:]).Msg("skipping unknown block")
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get block by ID (%s): %w", blockID, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		c.log.Debug().Msg("skipping empty batch response")
		return nil, nil
	}

	// send the response
	return &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}, nil
}
