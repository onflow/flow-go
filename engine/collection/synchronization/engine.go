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
	commonsync "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/metrics"
	synccore "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
)

// defaultSyncResponseQueueCapacity maximum capacity of sync responses queue
const defaultSyncResponseQueueCapacity = 500

// defaultBatchResponseQueueCapacity maximum capacity of batch responses queue
const defaultBatchResponseQueueCapacity = 500

// defaultRangeResponseQueueCapacity maximum capacity of range responses queue
const defaultRangeResponseQueueCapacity = 500

// Engine is the synchronization engine, responsible for synchronizing chain state.
type Engine struct {
	unit         *engine.Unit
	lm           *lifecycle.LifecycleManager
	log          zerolog.Logger
	metrics      module.EngineMetrics
	me           module.Local
	participants flow.IdentityList
	con          network.Conduit
	comp         network.Engine // compliance layer engine

	pollInterval time.Duration
	scanInterval time.Duration
	core         module.SyncCore
	state        cluster.State

	requestHandler *RequestHandlerEngine // component responsible for handling requests

	pendingSyncResponses   engine.MessageStore    // message store for *message.SyncResponse
	pendingBatchResponses  engine.MessageStore    // message store for *message.BatchResponse
	pendingRangeResponses  engine.MessageStore    // message store for *message.RangeResponse
	responseMessageHandler *engine.MessageHandler // message handler responsible for response processing
}

// New creates a new cluster chain synchronization engine.
func New(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net network.Network,
	me module.Local,
	participants flow.IdentityList,
	state cluster.State,
	blocks storage.ClusterBlocks,
	comp network.Engine,
	core module.SyncCore,
	opts ...commonsync.OptionFunc,
) (*Engine, error) {

	opt := commonsync.DefaultConfig()
	for _, f := range opts {
		f(opt)
	}

	if comp == nil {
		return nil, fmt.Errorf("must initialize synchronization engine with comp engine")
	}

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:         engine.NewUnit(),
		lm:           lifecycle.NewLifecycleManager(),
		log:          log.With().Str("engine", "cluster_synchronization").Logger(),
		metrics:      metrics,
		me:           me,
		participants: participants.Filter(filter.Not(filter.HasNodeID(me.NodeID()))),
		comp:         comp,
		core:         core,
		pollInterval: opt.PollInterval,
		scanInterval: opt.ScanInterval,
		state:        state,
	}

	err := e.setupResponseMessageHandler()
	if err != nil {
		return nil, fmt.Errorf("could not setup message handler")
	}

	chainID, err := state.Params().ChainID()
	if err != nil {
		return nil, fmt.Errorf("could not get chain ID: %w", err)
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ChannelSyncCluster(chainID), e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	e.requestHandler = NewRequestHandlerEngine(log, metrics, con, me, blocks, core, state)

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

	batchResponseQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultBatchResponseQueueCapacity))
	if err != nil {
		return fmt.Errorf("failed to create queue for batch responses: %w", err)
	}

	e.pendingBatchResponses = &engine.FifoMessageStore{
		FifoQueue: batchResponseQueue,
	}

	rangeResponseQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultRangeResponseQueueCapacity))
	if err != nil {
		return fmt.Errorf("failed to create queue for range responses: %w", err)
	}

	e.pendingRangeResponses = &engine.FifoMessageStore{
		FifoQueue: rangeResponseQueue,
	}

	// define message queueing behaviour
	e.responseMessageHandler = engine.NewMessageHandler(
		e.log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.SyncResponse)
				if ok {
					e.metrics.MessageReceived(metrics.EngineClusterSynchronization, metrics.MessageSyncResponse)
				}
				return ok
			},
			Store: e.pendingSyncResponses,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.ClusterBatchResponse)
				if ok {
					e.metrics.MessageReceived(metrics.EngineClusterSynchronization, metrics.MessageBatchResponse)
				}
				return ok
			},
			Store: e.pendingBatchResponses,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.ClusterRangeResponse)
				if ok {
					e.metrics.MessageReceived(metrics.EngineClusterSynchronization, metrics.MessageRangeResponse)
				}
				return ok
			},
			Store: e.pendingRangeResponses,
		},
	)

	return nil
}

// Ready returns a ready channel that is closed once the engine has fully started.
func (e *Engine) Ready() <-chan struct{} {
	e.lm.OnStart(func() {
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
		return e.requestHandler.process(originID, event)
	case *messages.SyncResponse, *messages.ClusterBatchResponse, *messages.ClusterRangeResponse:
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
			e.metrics.MessageHandled(metrics.EngineClusterSynchronization, metrics.MessageSyncResponse)
			continue
		}

		msg, ok = e.pendingBatchResponses.Get()
		if ok {
			e.onBatchResponse(msg.OriginID, msg.Payload.(*messages.ClusterBatchResponse))
			e.metrics.MessageHandled(metrics.EngineClusterSynchronization, metrics.MessageBatchResponse)
			continue
		}

		msg, ok = e.pendingRangeResponses.Get()
		if ok {
			e.onRangeResponse(msg.OriginID, msg.Payload.(*messages.ClusterRangeResponse))
			e.metrics.MessageHandled(metrics.EngineClusterSynchronization, metrics.MessageRangeResponse)
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return
	}
}

// onSyncResponse processes a synchronization response.
func (e *Engine) onSyncResponse(originID flow.Identifier, res *messages.SyncResponse) {
	final, err := e.state.Final().Head()
	if err != nil {
		e.log.Error().Err(err).Msg("could not get last finalized header")
		return
	}
	e.core.HandleHeight(final, res.Height)
}

// onBatchResponse processes a response containing a specifically requested batch.
func (e *Engine) onBatchResponse(originID flow.Identifier, res *messages.ClusterBatchResponse) {
	// process the blocks one by one
	for _, block := range res.Blocks {
		if !e.core.HandleBlock(block.Header) {
			continue
		}
		synced := &events.SyncedClusterBlock{
			OriginID: originID,
			Block:    block,
		}
		e.comp.SubmitLocal(synced)
	}
}

// onRangeResponse processes a response containing a specifically requested range.
// TODO: Currently, we trust that the response is honest and only contains finalized
// blocks. In the future, we may consider keeping track of the responses received
// and slashing nodes which responded to range requests with blocks which don't get
// finalized.
func (e *Engine) onRangeResponse(originID flow.Identifier, res *messages.ClusterRangeResponse) {
	// process the blocks one by one
	for _, block := range res.Blocks {
		if !e.core.HandleFinalizedBlock(block.Header) {
			continue
		}
		synced := &events.SyncedClusterBlock{
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
			final, err := e.state.Final().Head()
			if err != nil {
				e.log.Fatal().Err(err).Msg("could not get last finalized header")
				continue
			}
			ranges, batches := e.core.ScanPending(final)
			e.sendRequests(ranges, batches)
		}
	}

	// some minor cleanup
	scan.Stop()
}

// pollHeight will send a synchronization request to three random nodes.
func (e *Engine) pollHeight() {
	head, err := e.state.Final().Head()
	if err != nil {
		e.log.Error().Err(err).Msg("could not get last finalized header")
		return
	}

	// send the request for synchronization
	req := &messages.SyncRequest{
		Nonce:  rand.Uint64(),
		Height: head.Height,
	}
	err = e.con.Multicast(req, synccore.DefaultPollNodes, e.participants.NodeIDs()...)
	if err != nil {
		e.log.Warn().Err(err).Msg("sending sync request to poll heights failed")
		return
	}
	e.metrics.MessageSent(metrics.EngineClusterSynchronization, metrics.MessageSyncRequest)
}

// sendRequests sends a request for each range and batch using consensus participants from last finalized snapshot.
func (e *Engine) sendRequests(ranges []flow.Range, batches []flow.Batch) {
	var errs *multierror.Error

	for _, ran := range ranges {
		req := &messages.RangeRequest{
			Nonce:      rand.Uint64(),
			FromHeight: ran.From,
			ToHeight:   ran.To,
		}
		err := e.con.Multicast(req, synccore.DefaultBlockRequestNodes, e.participants.NodeIDs()...)
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
		e.metrics.MessageSent(metrics.EngineClusterSynchronization, metrics.MessageRangeRequest)
	}

	for _, batch := range batches {
		req := &messages.BatchRequest{
			Nonce:    rand.Uint64(),
			BlockIDs: batch.BlockIDs,
		}
		err := e.con.Multicast(req, synccore.DefaultBlockRequestNodes, e.participants.NodeIDs()...)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not submit batch request: %w", err))
			continue
		}
		e.core.BatchRequested(batch)
		e.metrics.MessageSent(metrics.EngineClusterSynchronization, metrics.MessageBatchRequest)
	}

	if err := errs.ErrorOrNil(); err != nil {
		e.log.Warn().Err(err).Msg("sending range and batch requests failed")
	}
}
