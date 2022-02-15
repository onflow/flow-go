// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	identifier "github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/metrics"
	synccore "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
)

// defaultSyncResponseQueueCapacity maximum capacity of sync responses queue
const defaultSyncResponseQueueCapacity = 500

// defaultBlockResponseQueueCapacity maximum capacity of block responses queue
const defaultBlockResponseQueueCapacity = 500

// Engine is the synchronization engine, responsible for synchronizing chain state.
type Engine struct {
	unit    *engine.Unit
	lm      *lifecycle.LifecycleManager
	log     zerolog.Logger
	metrics module.EngineMetrics
	me      module.Local
	con     network.Conduit
	blocks  storage.Blocks
	comp    network.Engine // compliance layer engine

	pollInterval         time.Duration
	scanInterval         time.Duration
	core                 module.SyncCore
	participantsProvider identifier.IdentifierProvider
	finalizedHeader      *FinalizedHeaderCache

	requestHandler *RequestHandler // component responsible for handling requests

	pendingSyncResponses   engine.MessageStore    // message store for *message.SyncResponse
	pendingBlockResponses  engine.MessageStore    // message store for *message.BlockResponse
	responseMessageHandler *engine.MessageHandler // message handler responsible for response processing
}

// New creates a new main chain synchronization engine.
func New(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net network.Network,
	me module.Local,
	blocks storage.Blocks,
	comp network.Engine,
	core module.SyncCore,
	finalizedHeader *FinalizedHeaderCache,
	participantsProvider identifier.IdentifierProvider,
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
		unit:                 engine.NewUnit(),
		lm:                   lifecycle.NewLifecycleManager(),
		log:                  log.With().Str("engine", "synchronization").Logger(),
		metrics:              metrics,
		me:                   me,
		blocks:               blocks,
		comp:                 comp,
		core:                 core,
		pollInterval:         opt.PollInterval,
		scanInterval:         opt.ScanInterval,
		finalizedHeader:      finalizedHeader,
		participantsProvider: participantsProvider,
	}

	err := e.setupResponseMessageHandler()
	if err != nil {
		return nil, fmt.Errorf("could not setup message handler")
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.SyncCommittee, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	e.requestHandler = NewRequestHandler(log, metrics, NewResponseSender(con), me, blocks, core, finalizedHeader, true)

	return e, nil
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

// Ready returns a ready channel that is closed once the engine has fully started.
func (e *Engine) Ready() <-chan struct{} {
	e.lm.OnStart(func() {
		<-e.finalizedHeader.Ready()
		e.unit.Launch(e.checkLoop)
		e.unit.Launch(e.responseProcessingLoop)
		// wait for request handler to startup
		<-e.requestHandler.Ready()
	})
	return e.lm.Started()
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	e.lm.OnStop(func() {
		// signal the request handler to shutdown
		requestHandlerDone := e.requestHandler.Done()
		// wait for request sending and response processing routines to exit
		<-e.unit.Done()
		// wait for request handler shutdown to complete
		<-requestHandlerDone
		<-e.finalizedHeader.Done()
	})
	return e.lm.Stopped()
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
	err := e.Process(channel, originID, event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	err := e.process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
			return nil
		}
		return fmt.Errorf("unexpected error while processing engine message: %w", err)
	}
	return nil
}

// process processes events for the synchronization engine.
// Error returns:
//  * IncompatibleInputTypeError if input has unexpected type
//  * All other errors are potential symptoms of internal state corruption or bugs (fatal).
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch event.(type) {
	case *messages.RangeRequest, *messages.BatchRequest, *messages.SyncRequest:
		return e.requestHandler.process(originID, event)
	case *messages.SyncResponse, *messages.BlockResponse:
		return e.responseMessageHandler.Process(originID, event)
	default:
		return fmt.Errorf("received input with type %T from %x: %w", event, originID[:], engine.IncompatibleInputTypeError)
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

// onSyncResponse processes a synchronization response.
func (e *Engine) onSyncResponse(originID flow.Identifier, res *messages.SyncResponse) {
	e.log.Debug().Str("origin_id", originID.String()).Msg("received sync response")
	final := e.finalizedHeader.Get()
	e.core.HandleHeight(final, res.Height)
}

// onBlockResponse processes a response containing a specifically requested block.
func (e *Engine) onBlockResponse(originID flow.Identifier, res *messages.BlockResponse) {
	// process the blocks one by one
	if len(res.Blocks) == 0 {
		e.log.Debug().Msg("received empty block response")
		return
	}

	first := res.Blocks[0].Header.Height
	last := res.Blocks[len(res.Blocks)-1].Header.Height
	e.log.Debug().Uint64("first", first).Uint64("last", last).Msg("received block response")

	for _, block := range res.Blocks {
		if !e.core.HandleBlock(block.Header) {
			e.log.Debug().Uint64("height", block.Header.Height).Msg("block handler rejected")
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
			head := e.finalizedHeader.Get()
			participants := e.participantsProvider.Identifiers()
			ranges, batches := e.core.ScanPending(head)
			e.sendRequests(participants, ranges, batches)
		}
	}

	// some minor cleanup
	scan.Stop()
}

// pollHeight will send a synchronization request to three random nodes.
func (e *Engine) pollHeight() {
	head := e.finalizedHeader.Get()
	participants := e.participantsProvider.Identifiers()

	// send the request for synchronization
	req := &messages.SyncRequest{
		Nonce:  rand.Uint64(),
		Height: head.Height,
	}
	e.log.Debug().
		Uint64("height", req.Height).
		Uint64("range_nonce", req.Nonce).
		Msg("sending sync request")
	err := e.con.Multicast(req, synccore.DefaultPollNodes, participants...)
	if err != nil {
		e.log.Warn().Err(err).Msg("sending sync request to poll heights failed")
		return
	}
	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageSyncRequest)
}

// sendRequests sends a request for each range and batch using consensus participants from last finalized snapshot.
func (e *Engine) sendRequests(participants flow.IdentifierList, ranges []flow.Range, batches []flow.Batch) {
	var errs *multierror.Error

	for _, ran := range ranges {
		req := &messages.RangeRequest{
			Nonce:      rand.Uint64(),
			FromHeight: ran.From,
			ToHeight:   ran.To,
		}
		err := e.con.Multicast(req, synccore.DefaultBlockRequestNodes, participants...)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not submit range request: %w", err))
			continue
		}
		e.log.Info().
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
		err := e.con.Multicast(req, synccore.DefaultBlockRequestNodes, participants...)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not submit batch request: %w", err))
			continue
		}
		e.log.Debug().
			Strs("block_ids", flow.IdentifierList(batch.BlockIDs).Strings()).
			Uint64("range_nonce", req.Nonce).
			Msg("batch requested")
		e.core.BatchRequested(batch)
		e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageBatchRequest)
	}

	if err := errs.ErrorOrNil(); err != nil {
		e.log.Warn().Err(err).Msg("sending range and batch requests failed")
	}
}
