// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/chainsync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	synccore "github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/events"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/rand"
)

// defaultSyncResponseQueueCapacity maximum capacity of sync responses queue
const defaultSyncResponseQueueCapacity = 500

// defaultBlockResponseQueueCapacity maximum capacity of block responses queue
const defaultBlockResponseQueueCapacity = 500

// Engine is the synchronization engine, responsible for synchronizing chain state.
type Engine struct {
	component.Component
	hotstuff.FinalizationConsumer

	log                  zerolog.Logger
	metrics              module.EngineMetrics
	me                   module.Local
	finalizedHeaderCache module.FinalizedHeaderCache
	con                  network.Conduit
	blocks               storage.Blocks
	comp                 consensus.Compliance

	pollInterval         time.Duration
	scanInterval         time.Duration
	core                 module.SyncCore
	participantsProvider module.IdentifierProvider

	requestHandler *RequestHandler // component responsible for handling requests

	pendingSyncResponses   engine.MessageStore    // message store for *message.SyncResponse
	pendingBlockResponses  engine.MessageStore    // message store for *message.BlockResponse
	responseMessageHandler *engine.MessageHandler // message handler responsible for response processing
}

var _ network.MessageProcessor = (*Engine)(nil)
var _ component.Component = (*Engine)(nil)

// New creates a new main chain synchronization engine.
func New(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net network.Network,
	me module.Local,
	state protocol.State,
	blocks storage.Blocks,
	comp consensus.Compliance,
	core module.SyncCore,
	participantsProvider module.IdentifierProvider,
	opts ...OptionFunc,
) (*Engine, error) {

	opt := DefaultConfig()
	for _, f := range opts {
		f(opt)
	}

	if comp == nil {
		panic("must initialize synchronization engine with comp engine")
	}

	finalizedHeaderCache, finalizedCacheWorker, err := events.NewFinalizedHeaderCache(state)
	if err != nil {
		return nil, fmt.Errorf("could not create finalized header cache: %w", err)
	}

	// initialize the propagation engine with its dependencies
	e := &Engine{
		FinalizationConsumer: finalizedHeaderCache,
		log:                  log.With().Str("engine", "synchronization").Logger(),
		metrics:              metrics,
		me:                   me,
		finalizedHeaderCache: finalizedHeaderCache,
		blocks:               blocks,
		comp:                 comp,
		core:                 core,
		pollInterval:         opt.PollInterval,
		scanInterval:         opt.ScanInterval,
		participantsProvider: participantsProvider,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(channels.SyncCommittee, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con
	e.requestHandler = NewRequestHandler(log, metrics, NewResponseSender(con), me, finalizedHeaderCache, blocks, core, true)

	// set up worker routines
	builder := component.NewComponentManagerBuilder().
		AddWorker(finalizedCacheWorker).
		AddWorker(e.checkLoop).
		AddWorker(e.responseProcessingLoop)
	for i := 0; i < defaultEngineRequestsWorkers; i++ {
		builder.AddWorker(e.requestHandler.requestProcessingWorker)
	}
	e.Component = builder.Build()

	err = e.setupResponseMessageHandler()
	if err != nil {
		return nil, fmt.Errorf("could not setup message handler")
	}

	return e, nil
}

// setupResponseMessageHandler initializes the inbound queues and the MessageHandler for UNTRUSTED responses.
func (e *Engine) setupResponseMessageHandler() error {
	syncResponseQueue, err := fifoqueue.NewFifoQueue(defaultSyncResponseQueueCapacity)
	if err != nil {
		return fmt.Errorf("failed to create queue for sync responses: %w", err)
	}

	e.pendingSyncResponses = &engine.FifoMessageStore{
		FifoQueue: syncResponseQueue,
	}

	blockResponseQueue, err := fifoqueue.NewFifoQueue(defaultBlockResponseQueueCapacity)
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

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	err := e.process(channel, originID, event)
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
//   - IncompatibleInputTypeError if input has unexpected type
//   - All other errors are potential symptoms of internal state corruption or bugs (fatal).
func (e *Engine) process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	switch event.(type) {
	case *messages.RangeRequest, *messages.BatchRequest, *messages.SyncRequest:
		return e.requestHandler.Process(channel, originID, event)
	case *messages.SyncResponse, *messages.BlockResponse:
		return e.responseMessageHandler.Process(originID, event)
	default:
		return fmt.Errorf("received input with type %T from %x: %w", event, originID[:], engine.IncompatibleInputTypeError)
	}
}

// responseProcessingLoop is a separate goroutine that performs processing of queued responses
func (e *Engine) responseProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	notifier := e.responseMessageHandler.GetNotifier()
	done := ctx.Done()
	for {
		select {
		case <-done:
			return
		case <-notifier:
			e.processAvailableResponses(ctx)
		}
	}
}

// processAvailableResponses is processor of pending events which drives events from networking layer to business logic.
func (e *Engine) processAvailableResponses(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
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
	final := e.finalizedHeaderCache.Get()
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

	filteredBlocks := make([]*messages.BlockProposal, 0, len(res.Blocks))
	for _, block := range res.Blocks {
		header := block.Header
		if !e.core.HandleBlock(&header) {
			e.log.Debug().Uint64("height", header.Height).Msg("block handler rejected")
			continue
		}
		filteredBlocks = append(filteredBlocks, &messages.BlockProposal{Block: block})
	}

	// forward the block to the compliance engine for validation and processing
	e.comp.OnSyncedBlocks(flow.Slashable[[]*messages.BlockProposal]{
		OriginID: originID,
		Message:  filteredBlocks,
	})
}

// checkLoop will regularly scan for items that need requesting.
func (e *Engine) checkLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	pollChan := make(<-chan time.Time)
	if e.pollInterval > 0 {
		poll := time.NewTicker(e.pollInterval)
		pollChan = poll.C
		defer poll.Stop()
	}
	scan := time.NewTicker(e.scanInterval)
	defer scan.Stop()

	done := ctx.Done()
	for {
		// give the quit channel a priority to be selected
		select {
		case <-done:
			return
		default:
		}

		select {
		case <-done:
			return
		case <-pollChan:
			e.pollHeight()
		case <-scan.C:
			final := e.finalizedHeaderCache.Get()
			participants := e.participantsProvider.Identifiers()
			ranges, batches := e.core.ScanPending(final)
			e.sendRequests(participants, ranges, batches)
		}
	}
}

// pollHeight will send a synchronization request to three random nodes.
func (e *Engine) pollHeight() {
	final := e.finalizedHeaderCache.Get()
	participants := e.participantsProvider.Identifiers()

	nonce, err := rand.Uint64()
	if err != nil {
		// TODO: this error should be returned by pollHeight()
		// it is logged for now since the only error possible is related to a failure
		// of the system entropy generation. Such error is going to cause failures in other
		// components where it's handled properly and will lead to crashing the module.
		e.log.Warn().Err(err).Msg("nonce generation failed during pollHeight")
		return
	}

	// send the request for synchronization
	req := &messages.SyncRequest{
		Nonce:  nonce,
		Height: final.Height,
	}
	e.log.Debug().
		Uint64("height", req.Height).
		Uint64("range_nonce", req.Nonce).
		Msg("sending sync request")
	err = e.con.Multicast(req, synccore.DefaultPollNodes, participants...)
	if err != nil {
		e.log.Warn().Err(err).Msg("sending sync request to poll heights failed")
		return
	}
	e.metrics.MessageSent(metrics.EngineSynchronization, metrics.MessageSyncRequest)
}

// sendRequests sends a request for each range and batch using consensus participants from last finalized snapshot.
func (e *Engine) sendRequests(participants flow.IdentifierList, ranges []chainsync.Range, batches []chainsync.Batch) {
	var errs *multierror.Error

	for _, ran := range ranges {
		nonce, err := rand.Uint64()
		if err != nil {
			// TODO: this error should be returned by sendRequests
			// it is logged for now since the only error possible is related to a failure
			// of the system entropy generation. Such error is going to cause failures in other
			// components where it's handled properly and will lead to crashing the module.
			e.log.Error().Err(err).Msg("nonce generation failed during range request")
			return
		}
		req := &messages.RangeRequest{
			Nonce:      nonce,
			FromHeight: ran.From,
			ToHeight:   ran.To,
		}
		err = e.con.Multicast(req, synccore.DefaultBlockRequestNodes, participants...)
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
		nonce, err := rand.Uint64()
		if err != nil {
			// TODO: this error should be returned by sendRequests
			// it is logged for now since the only error possible is related to a failure
			// of the system entropy generation. Such error is going to cause failures in other
			// components where it's handled properly and will lead to crashing the module.
			e.log.Error().Err(err).Msg("nonce generation failed during batch request")
			return
		}
		req := &messages.BatchRequest{
			Nonce:    nonce,
			BlockIDs: batch.BlockIDs,
		}
		err = e.con.Multicast(req, synccore.DefaultBlockRequestNodes, participants...)
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
