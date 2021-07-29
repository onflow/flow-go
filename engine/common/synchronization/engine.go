// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	synccore "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// defaultSyncRequestQueueCapacity maximum capacity of sync requests queue
const defaultSyncRequestQueueCapacity = 500

// defaultSyncRequestQueueCapacity maximum capacity of range requests queue
const defaultRangeRequestQueueCapacity = 500

// defaultSyncRequestQueueCapacity maximum capacity of batch requests queue
const defaultBatchRequestQueueCapacity = 500

// defaultSyncResponseQueueCapacity maximum capacity of sync responses queue
const defaultSyncResponseQueueCapacity = 500

// defaultBlockResponseQueueCapacity maximum capacity of block responses queue
const defaultBlockResponseQueueCapacity = 500

// defaultEngineRequestsWorkers number of workers to dispatch events for requests
const defaultEngineRequestsWorkers = 8

// finalSnapshot is a helper structure which contains latest finalized header and participants list
// for consensus nodes, it is used in Engine to access latest valid data
type finalizedSnapshot struct {
	head         *flow.Header
	participants flow.IdentityList
}

// Engine is the synchronization engine, responsible for synchronizing chain state.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	metrics module.EngineMetrics
	me      module.Local
	state   protocol.State
	con     network.Conduit
	blocks  storage.Blocks
	comp    network.Engine // compliance layer engine

	pollInterval          time.Duration
	scanInterval          time.Duration
	core                  module.SyncCore
	lastFinalizedSnapshot *finalizedSnapshot // last finalized snapshot of header and consensus participants

	pendingSyncRequests   engine.MessageStore    // message store for *message.SyncRequest
	pendingBatchRequests  engine.MessageStore    // message store for *message.BatchRequest
	pendingRangeRequests  engine.MessageStore    // message store for *message.RangeRequest
	requestMessageHandler *engine.MessageHandler // message handler responsible for request processing

	finalizationEventNotifier engine.Notifier // notifier for finalization events

	pendingSyncResponses   engine.MessageStore    // message store for *message.SyncResponse
	pendingBlockResponses  engine.MessageStore    // message store for *message.BlockResponse
	responseMessageHandler *engine.MessageHandler // message handler responsible for response processing
}

// New creates a new main chain synchronization engine.
func New(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net module.Network,
	me module.Local,
	state protocol.State,
	blocks storage.Blocks,
	comp network.Engine,
	core module.SyncCore,
	opts ...OptionFunc,
) (*Engine, error) {

	opt := DefaultConfig()
	for _, f := range opts {
		f(opt)
	}

	if comp == nil {
		panic("must initialize synchronization engine with comp engine")
	}

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:                      engine.NewUnit(),
		log:                       log.With().Str("engine", "synchronization").Logger(),
		metrics:                   metrics,
		me:                        me,
		state:                     state,
		blocks:                    blocks,
		comp:                      comp,
		core:                      core,
		pollInterval:              opt.pollInterval,
		scanInterval:              opt.scanInterval,
		finalizationEventNotifier: engine.NewNotifier(),
	}

	err := e.setupResponseMessageHandler()
	if err != nil {
		return nil, fmt.Errorf("could not setup message handler")
	}

	e.setupRequestMessageHandler()

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.SyncCommittee, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	err = e.onFinalizedBlock()
	if err != nil {
		return nil, fmt.Errorf("could not apply last finalized state")
	}

	return e, nil
}

// finalSnapshot returns last locally stored snapshot which contains final header
// and list of consensus participants
func (e *Engine) finalSnapshot() *finalizedSnapshot {
	e.unit.Lock()
	defer e.unit.Unlock()
	return e.lastFinalizedSnapshot
}

// onFinalizedBlock updates latest locally cached finalized snapshot
func (e *Engine) onFinalizedBlock() error {
	finalSnapshot := e.state.Final()
	head, err := finalSnapshot.Head()
	if err != nil {
		return fmt.Errorf("could not get last finalized header: %w", err)
	}

	// get all of the consensus nodes from the state
	participants, err := finalSnapshot.Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could get consensus participants at latest finalized block: %w", err)
	}

	e.unit.Lock()
	defer e.unit.Unlock()
	if e.lastFinalizedSnapshot != nil && e.lastFinalizedSnapshot.head.Height >= head.Height {
		return nil
	}
	e.lastFinalizedSnapshot = &finalizedSnapshot{
		head:         head,
		participants: participants,
	}
	return nil
}

// setupRequestMessageHandler initializes the inbound queues and the MessageHandler for UNTRUSTED requests.
func (e *Engine) setupRequestMessageHandler() {
	// RequestHeap deduplicates requests by keeping only one sync request for each requester.
	e.pendingSyncRequests = NewRequestHeap(defaultSyncRequestQueueCapacity)
	e.pendingRangeRequests = NewRequestHeap(defaultRangeRequestQueueCapacity)
	e.pendingBatchRequests = NewRequestHeap(defaultBatchRequestQueueCapacity)

	// define message queueing behaviour
	e.requestMessageHandler = engine.NewMessageHandler(
		e.log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.SyncRequest)
				if ok {
					e.metrics.MessageReceived(metrics.EngineSynchronization, metrics.MessageSyncRequest)
				}
				return ok
			},
			Store: e.pendingSyncRequests,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.RangeRequest)
				if ok {
					e.metrics.MessageReceived(metrics.EngineSynchronization, metrics.MessageRangeRequest)
				}
				return ok
			},
			Store: e.pendingRangeRequests,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.BatchRequest)
				if ok {
					e.metrics.MessageReceived(metrics.EngineSynchronization, metrics.MessageBatchRequest)
				}
				return ok
			},
			Store: e.pendingBatchRequests,
		},
	)
}

// setupResponseMessageHandler initializes the inbound queues and the MessageHandler for UNTRUSTED responses.
func (e *Engine) setupResponseMessageHandler() error {
	syncResponseQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultSyncResponseQueueCapacity))
	if err != nil {
		return fmt.Errorf("failed to create queue for sync responses: %w", err)
	}

	e.pendingSyncResponses = &engine.FifoMessageStore{
		FifoQueue: syncResponseQueue,
	}

	blockResponseQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultBlockResponseQueueCapacity))
	if err != nil {
		return fmt.Errorf("failed to create queue for block responses: %w", err)
	}

	e.pendingBlockResponses = &engine.FifoMessageStore{
		FifoQueue: blockResponseQueue,
	}

	// define message queueing behaviour
	e.responseMessageHandler = engine.NewMessageHandler(
		e.log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.SyncResponse)
				if ok {
					e.metrics.MessageReceived(metrics.EngineSynchronization, metrics.MessageSyncResponse)
				}
				return ok
			},
			Store: e.pendingSyncResponses,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.BlockResponse)
				if ok {
					e.metrics.MessageReceived(metrics.EngineSynchronization, metrics.MessageBlockResponse)
				}
				return ok
			},
			Store: e.pendingBlockResponses,
		},
	)

	return nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.checkLoop)
	for i := 0; i < defaultEngineRequestsWorkers; i++ {
		e.unit.Launch(e.requestProcessingLoop)
	}
	e.unit.Launch(e.responseProcessingLoop)
	e.unit.Launch(e.finalizationProcessingLoop)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	err := e.process(e.me.NodeID(), event)
	if err != nil {
		// receiving an input of incompatible type from a trusted internal component is fatal
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	err := e.process(originID, event)
	if err != nil {
		lg := e.log.With().
			Err(err).
			Str("channel", channel.String()).
			Str("origin", originID.String()).
			Logger()
		if errors.Is(err, engine.IncompatibleInputTypeError) {
			lg.Error().Msg("received message with incompatible type")
			return
		}
		lg.Fatal().Msg("internal error processing message")
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.process(originID, event)
}

// process processes events for the synchronization engine.
// Error returns:
//  * IncompatibleInputTypeError if input has unexpected type
//  * All other errors are potential symptoms of internal state corruption or bugs (fatal).
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch event.(type) {
	case *messages.RangeRequest, *messages.BatchRequest, *messages.SyncRequest:
		return e.requestMessageHandler.Process(originID, event)
	case *messages.SyncResponse, *messages.BlockResponse:
		return e.responseMessageHandler.Process(originID, event)
	default:
		return fmt.Errorf("received input with type %T from %x: %w", event, originID[:], engine.IncompatibleInputTypeError)
	}
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
//  (1) Updates local state of last finalized snapshot.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(flow.Identifier) {
	// notify that there is new finalized block
	e.finalizationEventNotifier.Notify()
}

// requestProcessingLoop is a separate goroutine that performs processing of queued requests
func (e *Engine) requestProcessingLoop() {
	notifier := e.requestMessageHandler.GetNotifier()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-notifier:
			err := e.processAvailableRequests()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing queued requests")
			}
		}
	}
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop() {
	notifier := e.finalizationEventNotifier.Channel()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-notifier:
			err := e.onFinalizedBlock()
			if err != nil {
				e.log.Fatal().Err(err).Msg("could not process latest finalized block")
			}
		}
	}
}

// responseProcessingLoop is a separate goroutine that performs processing of queued responses
func (e *Engine) responseProcessingLoop() {
	notifier := e.responseMessageHandler.GetNotifier()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-notifier:
			e.processAvailableResponses()
		}
	}
}

// processAvailableResponses is processor of pending events which drives events from networking layer to business logic.
func (e *Engine) processAvailableResponses() {
	for {
		select {
		case <-e.unit.Quit():
			return
		default:
		}

		msg, ok := e.pendingSyncResponses.Get()
		if ok {
			e.onSyncResponse(msg.OriginID, msg.Payload.(*messages.SyncResponse))
			e.metrics.MessageHandled(metrics.EngineSynchronization, metrics.MessageSyncResponse)
			continue
		}

		msg, ok = e.pendingBlockResponses.Get()
		if ok {
			e.onBlockResponse(msg.OriginID, msg.Payload.(*messages.BlockResponse))
			e.metrics.MessageHandled(metrics.EngineSynchronization, metrics.MessageBlockResponse)
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return
	}
}

// processAvailableRequests is processor of pending events which drives events from networking layer to business logic.
func (e *Engine) processAvailableRequests() error {
	for {
		select {
		case <-e.unit.Quit():
			return nil
		default:
		}

		msg, ok := e.pendingSyncRequests.Get()
		if ok {
			err := e.onSyncRequest(msg.OriginID, msg.Payload.(*messages.SyncRequest))
			if err != nil {
				return fmt.Errorf("processing sync request failed: %w", err)
			}
			continue
		}

		msg, ok = e.pendingRangeRequests.Get()
		if ok {
			err := e.onRangeRequest(msg.OriginID, msg.Payload.(*messages.RangeRequest))
			if err != nil {
				return fmt.Errorf("processing range request failed: %w", err)
			}
			continue
		}

		msg, ok = e.pendingBatchRequests.Get()
		if ok {
			err := e.onBatchRequest(msg.OriginID, msg.Payload.(*messages.BatchRequest))
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

// onSyncRequest processes an outgoing handshake; if we have a higher height, we
// inform the other node of it, so they can organize their block downloads. If
// we have a lower height, we add the difference to our own download queue.
func (e *Engine) onSyncRequest(originID flow.Identifier, req *messages.SyncRequest) error {
	final := e.finalSnapshot().head

	// queue any missing heights as needed
	e.core.HandleHeight(final, req.Height)

	// don't bother sending a response if we're within tolerance or if we're
	// behind the requester
	if e.core.WithinTolerance(final, req.Height) || req.Height > final.Height {
		return nil
	}

	// if we're sufficiently ahead of the requester, send a response
	res := &messages.SyncResponse{
		Height: final.Height,
		Nonce:  req.Nonce,
	}
	err := e.con.Unicast(res, originID)
	if err != nil {
		e.log.Warn().Err(err).Msg("sending sync response failed")
		return nil
	}
	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageSyncResponse)

	return nil
}

// onSyncResponse processes a synchronization response.
func (e *Engine) onSyncResponse(originID flow.Identifier, res *messages.SyncResponse) {
	final := e.finalSnapshot().head
	e.core.HandleHeight(final, res.Height)
}

// onRangeRequest processes a request for a range of blocks by height.
func (e *Engine) onRangeRequest(originID flow.Identifier, req *messages.RangeRequest) error {
	// get the latest final state to know if we can fulfill the request
	head := e.finalSnapshot().head

	// if we don't have anything to send, we can bail right away
	if head.Height < req.FromHeight || req.FromHeight > req.ToHeight {
		return nil
	}

	// get all of the blocks, one by one
	blocks := make([]*flow.Block, 0, req.ToHeight-req.FromHeight+1)
	for height := req.FromHeight; height <= req.ToHeight; height++ {
		block, err := e.blocks.ByHeight(height)
		if errors.Is(err, storage.ErrNotFound) {
			e.log.Error().Uint64("height", height).Msg("skipping unknown heights")
			break
		}
		if err != nil {
			return fmt.Errorf("could not get block for height (%d): %w", height, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		e.log.Debug().Msg("skipping empty range response")
		return nil
	}

	// send the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err := e.con.Unicast(res, originID)
	if err != nil {
		e.log.Warn().Err(err).Hex("origin_id", originID[:]).Msg("sending range response failed")
		return nil
	}
	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBlockResponse)

	return nil
}

// onBatchRequest processes a request for a specific block by block ID.
func (e *Engine) onBatchRequest(originID flow.Identifier, req *messages.BatchRequest) error {
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
		block, err := e.blocks.ByID(blockID)
		if errors.Is(err, storage.ErrNotFound) {
			e.log.Debug().Hex("block_id", blockID[:]).Msg("skipping unknown block")
			continue
		}
		if err != nil {
			return fmt.Errorf("could not get block by ID (%s): %w", blockID, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		e.log.Debug().Msg("skipping empty batch response")
		return nil
	}

	// send the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err := e.con.Unicast(res, originID)
	if err != nil {
		e.log.Warn().Err(err).Hex("origin_id", originID[:]).Msg("sending batch response failed")
		return nil
	}
	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBlockResponse)

	return nil
}

// onBlockResponse processes a response containing a specifically requested block.
func (e *Engine) onBlockResponse(originID flow.Identifier, res *messages.BlockResponse) {
	// process the blocks one by one
	for _, block := range res.Blocks {
		if !e.core.HandleBlock(block.Header) {
			continue
		}
		synced := &events.SyncedBlock{
			OriginID: originID,
			Block:    block,
		}
		e.comp.SubmitLocal(synced)
	}
}

// checkLoop will regularly scan for items that need requesting.
func (e *Engine) checkLoop() {
	pollChan := make(<-chan time.Time)
	if e.pollInterval > 0 {
		poll := time.NewTicker(e.pollInterval)
		pollChan = poll.C
		defer poll.Stop()
	}
	scan := time.NewTicker(e.scanInterval)

CheckLoop:
	for {
		// give the quit channel a priority to be selected
		select {
		case <-e.unit.Quit():
			break CheckLoop
		default:
		}

		select {
		case <-e.unit.Quit():
			break CheckLoop
		case <-pollChan:
			e.pollHeight()
		case <-scan.C:
			snapshot := e.finalSnapshot()
			ranges, batches := e.core.ScanPending(snapshot.head)
			e.sendRequests(snapshot.participants, ranges, batches)
		}
	}

	// some minor cleanup
	scan.Stop()
}

// pollHeight will send a synchronization request to three random nodes.
func (e *Engine) pollHeight() {
	snapshot := e.finalSnapshot()

	// send the request for synchronization
	req := &messages.SyncRequest{
		Nonce:  rand.Uint64(),
		Height: snapshot.head.Height,
	}
	err := e.con.Multicast(req, synccore.DefaultPollNodes, snapshot.participants.NodeIDs()...)
	if err != nil {
		e.log.Warn().Err(err).Msg("sending sync request to poll heights failed")
		return
	}
	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageSyncRequest)
}

// sendRequests sends a request for each range and batch using consensus participants from last finalized snapshot.
func (e *Engine) sendRequests(participants flow.IdentityList, ranges []flow.Range, batches []flow.Batch) {
	var errs *multierror.Error

	for _, ran := range ranges {
		req := &messages.RangeRequest{
			Nonce:      rand.Uint64(),
			FromHeight: ran.From,
			ToHeight:   ran.To,
		}
		err := e.con.Multicast(req, synccore.DefaultBlockRequestNodes, participants.NodeIDs()...)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not submit range request: %w", err))
			continue
		}
		e.log.Debug().
			Uint64("range_from", req.FromHeight).
			Uint64("range_to", req.ToHeight).
			Uint64("range_nonce", req.Nonce).
			Msg("range requested")
		e.core.RangeRequested(ran)
		e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageRangeRequest)
	}

	for _, batch := range batches {
		req := &messages.BatchRequest{
			Nonce:    rand.Uint64(),
			BlockIDs: batch.BlockIDs,
		}
		err := e.con.Multicast(req, synccore.DefaultBlockRequestNodes, participants.NodeIDs()...)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not submit batch request: %w", err))
			continue
		}
		e.core.BatchRequested(batch)
		e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBatchRequest)
	}

	if err := errs.ErrorOrNil(); err != nil {
		e.log.Warn().Err(err).Msg("sending range and batch requests failed")
	}
}
