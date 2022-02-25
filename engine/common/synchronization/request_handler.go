package synchronization

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/finalized_cache"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

type RequestHandler struct {
	blocks          storage.Blocks
	logger          zerolog.Logger
	finalizedHeader *finalized_cache.FinalizedHeaderCache
	maxResponseSize uint64
}

func NewRequestHandler(
	blocks storage.Blocks,
	logger zerolog.Logger,
	finalizedHeader *finalized_cache.FinalizedHeaderCache,
	maxResponseSize uint64,
) *RequestHandler {
	return &RequestHandler{
		blocks:          blocks,
		logger:          logger.With().Str("component", "request_handler").Logger(),
		finalizedHeader: finalizedHeader,
		maxResponseSize: maxResponseSize,
	}
}

func (s *RequestHandler) HandleRangeRequest(req *messages.RangeRequest) (resp *messages.BlockResponse, err error) {
	// get the local finalized height to determine if we can fulfill the request
	localHeight := s.finalizedHeader.Get().Height

	if localHeight < req.FromHeight {
		err = fmt.Errorf("local height %v is lower than requested range start height %v", localHeight, req.FromHeight)
		return
	}

	// limit response size
	maxHeight := req.FromHeight + uint64(s.maxResponseSize) - 1

	if maxHeight < req.ToHeight {
		req.ToHeight = maxHeight
	}

	blocks := make([]*flow.Block, 0, req.ToHeight-req.FromHeight+1)

	for height := req.FromHeight; height <= req.ToHeight; height++ {
		block, err := s.blocks.ByHeight(height)

		if err != nil {
			logger := s.logger.With().Uint64("height", height).Logger()

			if errors.Is(err, storage.ErrNotFound) {
				logger.Debug().Msg("skipping unknown heights")
			} else {
				logger.Err(err).Msg("failed to get block")
			}

			break
		}

		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		err = errors.New("empty range response")
		return
	}

	resp = &messages.BlockResponse{
		Blocks: blocks,
	}

	return
}

func (s *RequestHandler) HandleBatchRequest(req *messages.BatchRequest) (resp *messages.BlockResponse, err error) {
	// deduplicate the block IDs in the batch request
	blockIDs := make(map[flow.Identifier]struct{})

	for _, blockID := range req.BlockIDs {
		blockIDs[blockID] = struct{}{}

		// enforce client-side max request size
		if len(blockIDs) == int(s.maxResponseSize) {
			break
		}
	}

	blocks := make([]*flow.Block, 0, len(blockIDs))

	for blockID := range blockIDs {
		block, err := s.blocks.ByID(blockID)

		if err != nil {
			s.logger.Err(err).Str("block_id", blockID.String()).Msg("failed to get block")

			continue
		}

		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		err = errors.New("empty batch response")
		return
	}

	resp = &messages.BlockResponse{
		Blocks: blocks,
	}

	return
}

func (s *RequestHandler) HandleLatestFinalizedBlockRequest(req *messages.LatestFinalizedBlockRequest) *messages.LatestFinalizedBlockResponse {
	localHeader := s.finalizedHeader.Get()

	return &messages.LatestFinalizedBlockResponse{
		Height:  localHeader.Height,
		BlockID: localHeader.ID(),
	}
}
