package synchronization

import (
	"errors"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/rs/zerolog"
	"golang.org/x/net/context"
)

type RequestSender struct {
	config          *SenderConfig
	finalizedHeader *module.FinalizedHeaderCache
	logger          zerolog.Logger
	requestSender   network.RequestSender
	peerProvider    func() peer.IDSlice // TODO: filter out own ID
	core            module.SyncCore
	metrics         module.SyncMetrics
}

type SenderConfig struct {
	PollInterval        time.Duration
	ScanInterval        time.Duration
	NumPeersForRequest  uint
	MinNumRequests      uint
	MaxRequestSize      uint64
	MinRangeRequestSize uint64
	RequestTimeout      time.Duration
}

func (s *RequestSender) loop(ctx irrecoverable.SignalerContext) {
	poll := time.NewTicker(s.config.PollInterval)
	defer poll.Stop()

	scan := time.NewTicker(s.config.ScanInterval)
	defer scan.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			s.pollHeight()
		case <-scan.C:
			s.sendRequests()
		}
	}
}

func (s *RequestSender) sendRequests() {
	activeRange, batch := s.core.GetRequestableItems()

	numRequests := uint(0)

	for rangeStart := activeRange.From; activeRange.To-rangeStart >= s.config.MinRangeRequestSize-1; rangeStart += s.config.MaxRequestSize {

		rangeEnd := rangeStart + s.config.MaxRequestSize - 1

		if rangeEnd > activeRange.To {
			rangeEnd = activeRange.To
		}

		req := &messages.RangeRequest{
			FromHeight: rangeStart,
			ToHeight:   rangeEnd,
		}

		for _, target := range s.getTargets() {
			go s.sendRangeRequest(req, target)
		}

		numRequests++
	}

	for batchStart := 0; batchStart < len(batch.BlockIDs) && numRequests < s.config.MinNumRequests; batchStart += int(s.config.MaxRequestSize) {
		batchEnd := batchStart + int(s.config.MaxRequestSize)

		if batchEnd > len(batch.BlockIDs) {
			batchEnd = len(batch.BlockIDs)
		}

		req := &messages.BatchRequest{
			BlockIDs: batch.BlockIDs[batchStart:batchEnd],
		}

		for _, target := range s.getTargets() {
			go s.sendBatchRequest(req, target)
		}

		numRequests++
	}
}

func (s *RequestSender) sendRangeRequest(req *messages.RangeRequest, target peer.ID) {
	logger := s.logger.With().Str("target", target.String()).Logger()

	logger.Debug().
		Uint64("from_height", req.FromHeight).
		Uint64("to_height", req.ToHeight).
		Msg("sending range request")

	s.metrics.RangeRequestStarted()
	started := time.Now()
	success := false

	defer func() {
		s.metrics.RangeRequestFinished(time.Since(started), success)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), s.config.RequestTimeout)
	defer cancel()

	response, err := s.requestSender.SendRequest(ctx, req, target)
	cancel()

	if err != nil {
		logger.Err(err).
			Msg("failed to send range request")
		return
	}

	msg, ok := response.(*messages.BlockResponse)

	if !ok {
		logger.Debug().
			Interface("response", response).
			Msg("received invalid range response")
		return
	}

	blockIDs, err := s.validateRangeResponse(req, msg)

	if err != nil {
		// TODO

		return
	}

	s.core.RangeReceived(req.FromHeight, blockIDs, target)

	success = true
}

func (s *RequestSender) validateRangeResponse(req *messages.RangeRequest, resp *messages.BlockResponse) ([]flow.Identifier, error) {
	if len(resp.Blocks) == 0 {
		// TODO
		return nil, errors.New("empty response")
	}

	if len(resp.Blocks) > int(req.FromHeight-req.ToHeight+1) {
		// TODO
		return nil, errors.New("response too large")
	}

	blockIDs := make([]flow.Identifier, len(resp.Blocks))

	for i, block := range resp.Blocks {
		if block.Header.Height != req.FromHeight+uint64(i) {
			// TODO
			return nil, errors.New("invalid height")
		}

		if !block.Valid() {
			// TODO
			return nil, errors.New("invalid block")
		}

		blockIDs[i] = block.ID()
	}

	return blockIDs, nil
}

func (s *RequestSender) sendBatchRequest(req *messages.BatchRequest, target peer.ID) {
	logger := s.logger.With().Str("target", target.String()).Logger()

	blockIDsArr := zerolog.Arr()

	for _, blockID := range req.BlockIDs {
		blockIDsArr.Str(blockID.String())
	}

	logger.Debug().
		Array("block_ids", blockIDsArr).
		Msg("sending batch request")

	s.metrics.BatchRequestStarted()
	started := time.Now()
	success := false

	defer func() {
		s.metrics.BatchRequestFinished(time.Since(started), success)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), s.config.RequestTimeout)
	defer cancel()

	response, err := s.requestSender.SendRequest(ctx, req, target)
	cancel()

	if err != nil {
		logger.Err(err).
			Msg("failed to send batch request")
		return
	}

	msg, ok := response.(*messages.BlockResponse)

	if !ok {
		logger.Debug().
			Interface("response", response).
			Msg("received invalid range response")
		return
	}

	blockIDs, err := s.validateBatchResponse(req, msg)

	if err != nil {
		// TODO

		return
	}

	s.core.BatchReceived(blockIDs, target)

	success = true
}

func (s *RequestSender) validateBatchResponse(req *messages.BatchRequest, resp *messages.BlockResponse) ([]flow.Identifier, error) {
	if len(resp.Blocks) == 0 {
		// TODO
		return nil, errors.New("empty response")
	}

	if len(resp.Blocks) > len(req.BlockIDs) {
		// TODO
		return nil, errors.New("response too large")
	}

	blockIDs := make([]flow.Identifier, len(resp.Blocks))
	received := make(map[flow.Identifier]bool)

	for _, blockID := range req.BlockIDs {
		received[blockID] = false
	}

	for i, block := range resp.Blocks {
		duplicate, requested := received[block.ID()]

		if !requested {
			// TODO
			return nil, errors.New("invalid block id")
		}

		if duplicate {
			// TODO: duplicate block
			return nil, errors.New("duplicate block")
		}

		if !block.Valid() {
			// TODO
			return nil, errors.New("invalid block")
		}

		received[block.ID()] = true
		blockIDs[i] = block.ID()
	}

	return blockIDs, nil
}

func (s *RequestSender) getTargets() peer.IDSlice {
	peers := s.peerProvider()

	for i := 0; i < int(s.config.NumPeersForRequest); i++ {
		j := rand.Intn(len(peers) - i)
		peers[i], peers[j+1] = peers[j+1], peers[i]
	}

	return peers[:s.config.NumPeersForRequest]
}

func (s *RequestSender) pollHeight() {
	req := &messages.SyncRequest{
		Height: s.finalizedHeader.Get().Height,
	}

	for _, target := range s.getTargets() {
		go s.sendSyncHeightRequest(req, target)
	}
}

func (s *RequestSender) sendSyncHeightRequest(req *messages.SyncRequest, target peer.ID) {
	logger := s.logger.With().Str("target", target.String()).Logger()

	logger.Debug().
		Uint64("height", req.Height).
		Msg("sending sync height request")

	s.metrics.BatchRequestStarted()
	started := time.Now()
	success := false
	height := uint64(0)

	defer func() {
		s.metrics.SyncHeightRequestFinished(time.Since(started), success, height)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), s.config.RequestTimeout)
	defer cancel()

	response, err := s.requestSender.SendRequest(ctx, req, target)
	cancel()

	if err != nil {
		logger.Err(err).
			Msg("failed to send sync height request")
		return
	}

	msg, ok := response.(*messages.SyncResponse)

	if !ok {
		logger.Debug().
			Interface("response", response).
			Msg("received invalid sync height response")
		return
	}

	// TODO: validate response?

	height = msg.Height

	s.core.HeightReceived(msg.Height, target)

	success = true
}
