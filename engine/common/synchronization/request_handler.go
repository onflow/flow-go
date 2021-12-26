package synchronization

import (
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

type RequestHandler struct {
	blocks          storage.Blocks
	logger          zerolog.Logger
	finalizedHeader *module.FinalizedHeaderCache
	metrics         module.SyncMetrics
	config          *HandlerConfig
	core            module.SyncCore
}

type HandlerConfig struct {
	MaxResponseSize        uint64
	ProcessReceivedHeights bool
}

func (s *RequestHandler) HandleRequest(request interface{}, originID peer.ID) (interface{}, error) {
	switch r := request.(type) {
	case *messages.RangeRequest:
		return s.handleRangeRequest(r, originID)
	case *messages.BatchRequest:
		return s.handleBatchRequest(r, originID)
	case *messages.SyncRequest:
		return s.handleSyncRequest(r, originID), nil
	default:
		return nil, fmt.Errorf("received input with type %T from %v: %w", request, originID.String(), engine.IncompatibleInputTypeError)
	}
}

func (s *RequestHandler) handleRangeRequest(req *messages.RangeRequest, origin peer.ID) (resp *messages.BlockResponse, err error) {
	s.logger.Debug().Str("origin_id", origin.String()).Msg("received new range request")

	s.metrics.RangeResponseStarted()
	started := time.Now()

	defer func() {
		s.metrics.RangeResponseFinished(time.Since(started), err == nil)
	}()

	// get the local finalized height to determine if we can fulfill the request
	localHeight := s.finalizedHeader.Get().Height

	if localHeight < req.FromHeight {
		err = fmt.Errorf("local height %v is lower than requested range start height %v", localHeight, req.FromHeight)
		return
	}

	// enforce client-side max request size
	maxHeight := req.FromHeight + uint64(s.config.MaxResponseSize) - 1

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

func (s *RequestHandler) handleBatchRequest(req *messages.BatchRequest, origin peer.ID) (resp *messages.BlockResponse, err error) {
	s.logger.Debug().Str("origin_id", origin.String()).Msg("received new batch request")

	s.metrics.BatchResponseStarted()
	started := time.Now()

	// TODO!!! the success should not be based on whether or not the error is nil
	// similar for other defers
	defer func() {
		s.metrics.BatchResponseFinished(time.Since(started), err == 0)
	}()

	// deduplicate the block IDs in the batch request
	blockIDs := make(map[flow.Identifier]struct{})

	for _, blockID := range req.BlockIDs {
		blockIDs[blockID] = struct{}{}

		// enforce client-side max request size
		if len(blockIDs) == int(s.config.MaxResponseSize) {
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

func (s *RequestHandler) handleSyncRequest(req *messages.SyncRequest, origin peer.ID) *messages.SyncResponse {
	s.logger.Debug().Str("origin_id", origin.String()).Msg("received new sync request")

	s.metrics.SyncHeightResponseStarted()
	started := time.Now()

	defer func() {
		s.metrics.SyncHeightResponseFinished(time.Since(started), req.Height)
	}()

	if s.config.ProcessReceivedHeights {
		s.core.HeightReceived(req.Height, origin)
	}

	localHeight := s.finalizedHeader.Get().Height

	return &messages.SyncResponse{
		Height: localHeight,
	}
}
