package synchronization

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/events"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// defaultSyncRequestQueueCapacity maximum capacity of sync requests queue
const defaultSyncRequestQueueCapacity = 500

// defaultSyncRequestQueueCapacity maximum capacity of range requests queue
const defaultRangeRequestQueueCapacity = 500

// defaultSyncRequestQueueCapacity maximum capacity of batch requests queue
const defaultBatchRequestQueueCapacity = 500

// defaultEngineRequestsWorkers number of workers to dispatch events for requests
const defaultEngineRequestsWorkers = 8

// RequestHandler encapsulates message queues and processing logic for the sync engine.
// It logically separates request processing from active participation (sending requests),
// primarily to simplify nodes which bridge the public and private networks.
//
// The RequestHandlerEngine embeds RequestHandler to create an engine which only responds
// to requests on the public network (does not send requests over this network).
// The Engine embeds RequestHandler and additionally includes logic for sending sync requests.
//
// Although the RequestHandler defines a notifier, message queue, and processing worker logic,
// it is not itself a component.Component and does not manage any worker threads. The containing
// engine is responsible for starting the worker threads for processing requests.
type RequestHandler struct {
	me      module.Local
	log     zerolog.Logger
	metrics module.EngineMetrics

	blocks               storage.Blocks
	finalizedHeaderCache module.FinalizedHeaderCache
	core                 module.SyncCore
	responseSender       ResponseSender

	pendingSyncRequests   engine.MessageStore    // message store for *message.SyncRequest
	pendingBatchRequests  engine.MessageStore    // message store for *message.BatchRequest
	pendingRangeRequests  engine.MessageStore    // message store for *message.RangeRequest
	requestMessageHandler *engine.MessageHandler // message handler responsible for request processing

	queueMissingHeights bool // true if missing heights should be added to download queue
}

func NewRequestHandler(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	responseSender ResponseSender,
	me module.Local,
	finalizedHeaderCache *events.FinalizedHeaderCache,
	blocks storage.Blocks,
	core module.SyncCore,
	queueMissingHeights bool,
) *RequestHandler {
	r := &RequestHandler{
		me:                   me,
		log:                  log.With().Str("engine", "synchronization").Logger(),
		metrics:              metrics,
		finalizedHeaderCache: finalizedHeaderCache,
		blocks:               blocks,
		core:                 core,
		responseSender:       responseSender,
		queueMissingHeights:  queueMissingHeights,
	}

	r.setupRequestMessageHandler()

	return r
}

// Process processes the given event from the node with the given origin ID in a blocking manner.
// No errors are expected during normal operation.
func (r *RequestHandler) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	err := r.requestMessageHandler.Process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			r.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
			return nil
		}
		return fmt.Errorf("unexpected error while processing engine message: %w", err)
	}
	return nil
}

// setupRequestMessageHandler initializes the inbound queues and the MessageHandler for UNTRUSTED requests.
func (r *RequestHandler) setupRequestMessageHandler() {
	// RequestHeap deduplicates requests by keeping only one sync request for each requester.
	r.pendingSyncRequests = NewRequestHeap(defaultSyncRequestQueueCapacity)
	r.pendingRangeRequests = NewRequestHeap(defaultRangeRequestQueueCapacity)
	r.pendingBatchRequests = NewRequestHeap(defaultBatchRequestQueueCapacity)

	// define message queueing behaviour
	r.requestMessageHandler = engine.NewMessageHandler(
		r.log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.SyncRequest)
				if ok {
					r.metrics.MessageReceived(metrics.EngineSynchronization, metrics.MessageSyncRequest)
				}
				return ok
			},
			Store: r.pendingSyncRequests,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.RangeRequest)
				if ok {
					r.metrics.MessageReceived(metrics.EngineSynchronization, metrics.MessageRangeRequest)
				}
				return ok
			},
			Store: r.pendingRangeRequests,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.BatchRequest)
				if ok {
					r.metrics.MessageReceived(metrics.EngineSynchronization, metrics.MessageBatchRequest)
				}
				return ok
			},
			Store: r.pendingBatchRequests,
		},
	)
}

// onSyncRequest processes an outgoing handshake; if we have a higher height, we
// inform the other node of it, so they can organize their block downloads. If
// we have a lower height, we add the difference to our own download queue.
// No errors are expected during normal operation.
func (r *RequestHandler) onSyncRequest(originID flow.Identifier, req *messages.SyncRequest) error {
	finalizedHeader := r.finalizedHeaderCache.Get()

	logger := r.log.With().Str("origin_id", originID.String()).Logger()
	logger.Debug().
		Uint64("origin_height", req.Height).
		Uint64("local_height", finalizedHeader.Height).
		Msg("received new sync request")

	if r.queueMissingHeights {
		// queue any missing heights as needed
		r.core.HandleHeight(finalizedHeader, req.Height)
	}

	// don't bother sending a response if we're within tolerance or if we're
	// behind the requester
	if r.core.WithinTolerance(finalizedHeader, req.Height) || req.Height > finalizedHeader.Height {
		return nil
	}

	// if we're sufficiently ahead of the requester, send a response
	res := &messages.SyncResponse{
		Height: finalizedHeader.Height,
		Nonce:  req.Nonce,
	}
	err := r.responseSender.SendResponse(res, originID)
	if err != nil {
		logger.Warn().Err(err).Msg("sending sync response failed")
		return nil
	}
	r.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageSyncResponse)

	return nil
}

// onRangeRequest processes a request for a range of blocks by height.
// No errors are expected during normal operation.
func (r *RequestHandler) onRangeRequest(originID flow.Identifier, req *messages.RangeRequest) error {
	logger := r.log.With().Str("origin_id", originID.String()).Logger()
	logger.Debug().Msg("received new range request")

	// get the latest final state to know if we can fulfill the request
	finalizedHeader := r.finalizedHeaderCache.Get()

	// if we don't have anything to send, we can bail right away
	if finalizedHeader.Height < req.FromHeight || req.FromHeight > req.ToHeight {
		return nil
	}

	// enforce client-side max request size
	var maxSize uint
	// TODO: clean up this logic
	if core, ok := r.core.(*chainsync.Core); ok {
		maxSize = core.Config.MaxSize
	} else {
		maxSize = chainsync.DefaultConfig().MaxSize
	}
	maxHeight := req.FromHeight + uint64(maxSize)
	if maxHeight < req.ToHeight {
		logger.Warn().
			Uint64("from", req.FromHeight).
			Uint64("to", req.ToHeight).
			Uint64("size", (req.ToHeight-req.FromHeight)+1).
			Uint("max_size", maxSize).
			Bool(logging.KeySuspicious, true).
			Msg("range request is too large")

		req.ToHeight = maxHeight
	}

	// get all the blocks, one by one
	blocks := make([]messages.UntrustedBlock, 0, req.ToHeight-req.FromHeight+1)
	for height := req.FromHeight; height <= req.ToHeight; height++ {
		block, err := r.blocks.ByHeight(height)
		if errors.Is(err, storage.ErrNotFound) {
			logger.Error().Uint64("height", height).Msg("skipping unknown heights")
			break
		}
		if err != nil {
			return fmt.Errorf("could not get block for height (%d): %w", height, err)
		}
		blocks = append(blocks, messages.UntrustedBlockFromInternal(block))
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		logger.Debug().Msg("skipping empty range response")
		return nil
	}

	// send the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err := r.responseSender.SendResponse(res, originID)
	if err != nil {
		logger.Warn().Err(err).Msg("sending range response failed")
		return nil
	}
	r.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBlockResponse)

	return nil
}

// onBatchRequest processes a request for a specific block by block ID.
func (r *RequestHandler) onBatchRequest(originID flow.Identifier, req *messages.BatchRequest) error {
	logger := r.log.With().Str("origin_id", originID.String()).Logger()
	logger.Debug().Msg("received new batch request")

	// we should bail and send nothing on empty request
	if len(req.BlockIDs) == 0 {
		return nil
	}

	// TODO: clean up this logic
	var maxSize uint
	if core, ok := r.core.(*chainsync.Core); ok {
		maxSize = core.Config.MaxSize
	} else {
		maxSize = chainsync.DefaultConfig().MaxSize
	}

	if len(req.BlockIDs) > int(maxSize) {
		logger.Warn().
			Int("size", len(req.BlockIDs)).
			Uint("max_size", maxSize).
			Bool(logging.KeySuspicious, true).
			Msg("batch request is too large")
	}

	// deduplicate the block IDs in the batch request
	blockIDs := make(map[flow.Identifier]struct{})
	for _, blockID := range req.BlockIDs {
		blockIDs[blockID] = struct{}{}

		// enforce client-side max request size
		if len(blockIDs) == int(maxSize) {
			break
		}
	}

	// try to get all the blocks by ID
	blocks := make([]messages.UntrustedBlock, 0, len(blockIDs))
	for blockID := range blockIDs {
		block, err := r.blocks.ByID(blockID)
		if errors.Is(err, storage.ErrNotFound) {
			logger.Debug().Hex("block_id", blockID[:]).Msg("skipping unknown block")
			continue
		}
		if err != nil {
			return fmt.Errorf("could not get block by ID (%s): %w", blockID, err)
		}
		blocks = append(blocks, messages.UntrustedBlockFromInternal(block))
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		logger.Debug().Msg("skipping empty batch response")
		return nil
	}

	// send the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err := r.responseSender.SendResponse(res, originID)
	if err != nil {
		logger.Warn().Err(err).Msg("sending batch response failed")
		return nil
	}
	r.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBlockResponse)

	return nil
}

// processAvailableRequests is processor of pending events which drives events from networking layer to business logic.
func (r *RequestHandler) processAvailableRequests(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := r.pendingSyncRequests.Get()
		if ok {
			err := r.onSyncRequest(msg.OriginID, msg.Payload.(*messages.SyncRequest))
			if err != nil {
				return fmt.Errorf("processing sync request failed: %w", err)
			}
			continue
		}

		msg, ok = r.pendingRangeRequests.Get()
		if ok {
			err := r.onRangeRequest(msg.OriginID, msg.Payload.(*messages.RangeRequest))
			if err != nil {
				return fmt.Errorf("processing range request failed: %w", err)
			}
			continue
		}

		msg, ok = r.pendingBatchRequests.Get()
		if ok {
			err := r.onBatchRequest(msg.OriginID, msg.Payload.(*messages.BatchRequest))
			if err != nil {
				return fmt.Errorf("processing batch request failed: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// requestProcessingWorker is a separate goroutine that performs processing of queued requests.
// Multiple instances may be invoked. It is invoked and managed by the Engine or RequestHandlerEngine
// which embeds this RequestHandler.
func (r *RequestHandler) requestProcessingWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	notifier := r.requestMessageHandler.GetNotifier()
	done := ctx.Done()
	for {
		select {
		case <-done:
			return
		case <-notifier:
			err := r.processAvailableRequests(ctx)
			if err != nil {
				r.log.Err(err).Msg("internal error processing queued requests")
				ctx.Throw(err)
			}
		}
	}
}
