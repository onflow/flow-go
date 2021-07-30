package synchronization

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

type RequestHandler struct {
	blocks         storage.Blocks // TODO: function to get blocks?
	getFinalHeader func() *flow.Header
	updateHeight   func(final *flow.Header, height uint64)
	con            network.Conduit // Do we need this? or directly return result to caller?
	// If anything, we should probably pass in the entire network instance, so that
	// the handler can subscribe to messages at the same time

	pendingSyncRequests   engine.MessageStore    // message store for *message.SyncRequest
	pendingBatchRequests  engine.MessageStore    // message store for *message.BatchRequest
	pendingRangeRequests  engine.MessageStore    // message store for *message.RangeRequest
	requestMessageHandler *engine.MessageHandler // message handler responsible for request processing

	log     zerolog.Logger       // TODO: when this engine runs on unstaked network, we should use separate
	metrics module.EngineMetrics // TODO: values for these vs the one from staked network?
	// or maybe metrics.EngineSynchronization is what needs to change.

	module.Engine
}

func NewRequestHandler(getFinalHeader func() *flow.Header) *RequestHandler {
	return &RequestHandler{}
}

// TODO: note that on the staked network this only receives local submissions from sync engine
func (r *RequestHandler) process(originID flow.Identifier, event interface{}) error {
	switch event.(type) {
	case *messages.RangeRequest, *messages.BatchRequest, *messages.SyncRequest:
		return r.requestMessageHandler.Process(originID, event)
	default:
		return fmt.Errorf("received input with type %T from %x: %w", event, originID[:], engine.IncompatibleInputTypeError)
	}
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
func (r *RequestHandler) onSyncRequest(originID flow.Identifier, req *messages.SyncRequest) error {
	final := r.getFinalHeader()

	// queue any missing heights as needed
	r.core.HandleHeight(final, req.Height)

	// don't bother sending a response if we're within tolerance or if we're
	// behind the requester
	if r.core.WithinTolerance(final, req.Height) || req.Height > final.Height {
		return nil
	}

	// if we're sufficiently ahead of the requester, send a response
	res := &messages.SyncResponse{
		Height: final.Height,
		Nonce:  req.Nonce,
	}
	err := r.con.Unicast(res, originID)
	if err != nil {
		r.log.Warn().Err(err).Msg("sending sync response failed")
		return nil
	}
	r.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageSyncResponse)

	return nil
}

// onRangeRequest processes a request for a range of blocks by height.
func (r *RequestHandler) onRangeRequest(originID flow.Identifier, req *messages.RangeRequest) error {
	// get the latest final state to know if we can fulfill the request
	head := r.getFinalHeader()

	// if we don't have anything to send, we can bail right away
	if head.Height < req.FromHeight || req.FromHeight > req.ToHeight {
		return nil
	}

	// get all of the blocks, one by one
	blocks := make([]*flow.Block, 0, req.ToHeight-req.FromHeight+1)
	for height := req.FromHeight; height <= req.ToHeight; height++ {
		block, err := r.blocks.ByHeight(height)
		if errors.Is(err, storage.ErrNotFound) {
			r.log.Error().Uint64("height", height).Msg("skipping unknown heights")
			break
		}
		if err != nil {
			return fmt.Errorf("could not get block for height (%d): %w", height, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		r.log.Debug().Msg("skipping empty range response")
		return nil
	}

	// send the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err := r.con.Unicast(res, originID)
	if err != nil {
		r.log.Warn().Err(err).Hex("origin_id", originID[:]).Msg("sending range response failed")
		return nil
	}
	r.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBlockResponse)

	return nil
}

// onBatchRequest processes a request for a specific block by block ID.
func (r *RequestHandler) onBatchRequest(originID flow.Identifier, req *messages.BatchRequest) error {
	// we should bail and send nothing on empty request
	if len(req.BlockIDs) == 0 {
		return nil
	}

	// deduplicate the block IDs in the batch request
	blockIDs := make(map[flow.Identifier]struct{})
	for _, blockID := range req.BlockIDs {
		blockIDs[blockID] = struct{}{}
	}

	// try to get all the blocks by ID
	blocks := make([]*flow.Block, 0, len(blockIDs))
	for blockID := range blockIDs {
		block, err := r.blocks.ByID(blockID)
		if errors.Is(err, storage.ErrNotFound) {
			r.log.Debug().Hex("block_id", blockID[:]).Msg("skipping unknown block")
			continue
		}
		if err != nil {
			return fmt.Errorf("could not get block by ID (%s): %w", blockID, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		r.log.Debug().Msg("skipping empty batch response")
		return nil
	}

	// send the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err := r.con.Unicast(res, originID)
	if err != nil {
		r.log.Warn().Err(err).Hex("origin_id", originID[:]).Msg("sending batch response failed")
		return nil
	}
	r.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBlockResponse)

	return nil
}
