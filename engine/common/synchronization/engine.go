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
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
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

	requestHandler      *RequestHandler // component responsible for handling requests
	spamDetectionConfig *SpamDetectionConfig

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
	net network.EngineRegistry,
	me module.Local,
	state protocol.State,
	blocks storage.Blocks,
	comp consensus.Compliance,
	core module.SyncCore,
	participantsProvider module.IdentifierProvider,
	spamDetectionConfig *SpamDetectionConfig,
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
		spamDetectionConfig:  spamDetectionConfig,
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
	switch message := event.(type) {
	case *messages.BatchRequest:
		report, valid, err := e.validateBatchRequestForALSP(channel, originID, message)
		if err != nil {
			return fmt.Errorf("failed to validate batch request from %x: %w", originID[:], err)
		}
		if !valid {
			e.con.ReportMisbehavior(report) // report misbehavior to ALSP
			e.log.
				Warn().
				Hex("origin_id", logging.ID(originID)).
				Str(logging.KeySuspicious, "true").
				Msgf("received invalid batch request from %x: %v", originID[:], valid)
			e.metrics.InboundMessageDropped(metrics.EngineSynchronization, metrics.MessageBatchRequest)
			return nil
		}
		return e.requestHandler.Process(channel, originID, event)
	case *messages.RangeRequest:
		report, valid, err := e.validateRangeRequestForALSP(originID, message)
		if err != nil {
			return fmt.Errorf("failed to validate range request from %x: %w", originID[:], err)
		}
		if !valid {
			e.con.ReportMisbehavior(report) // report misbehavior to ALSP
			e.log.
				Warn().
				Hex("origin_id", logging.ID(originID)).
				Str(logging.KeySuspicious, "true").
				Msgf("received invalid range request from %x: %v", originID[:], valid)
			e.metrics.InboundMessageDropped(metrics.EngineSynchronization, metrics.MessageRangeRequest)
			return nil
		}
		return e.requestHandler.Process(channel, originID, event)

	case *messages.SyncRequest:
		report, valid, err := e.validateSyncRequestForALSP(originID)
		if err != nil {
			return fmt.Errorf("failed to validate sync request from %x: %w", originID[:], err)
		}
		if !valid {
			e.con.ReportMisbehavior(report) // report misbehavior to ALSP
			e.log.
				Warn().
				Hex("origin_id", logging.ID(originID)).
				Str(logging.KeySuspicious, "true").
				Msgf("received invalid sync request from %x: %v", originID[:], valid)
			e.metrics.InboundMessageDropped(metrics.EngineSynchronization, metrics.MessageSyncRequest)
			return nil
		}
		return e.requestHandler.Process(channel, originID, event)

	case *messages.BlockResponse:
		report, valid, err := e.validateBlockResponseForALSP(channel, originID, message)
		if err != nil {
			return fmt.Errorf("failed to validate block response from %x: %w", originID[:], err)
		}
		if !valid {
			e.con.ReportMisbehavior(report) // report misbehavior to ALSP
			e.log.
				Warn().
				Hex("origin_id", logging.ID(originID)).
				Str(logging.KeySuspicious, "true").
				Msgf("received invalid block response from %x: %v", originID[:], valid)
			e.metrics.InboundMessageDropped(metrics.EngineSynchronization, metrics.MessageBlockResponse)
			return nil
		}
		return e.responseMessageHandler.Process(originID, event)

	case *messages.SyncResponse:
		report, valid, err := e.validateSyncResponseForALSP(channel, originID, message)
		if err != nil {
			return fmt.Errorf("failed to validate sync response from %x: %w", originID[:], err)
		}
		if !valid {
			e.con.ReportMisbehavior(report) // report misbehavior to ALSP
			e.log.
				Warn().
				Hex("origin_id", logging.ID(originID)).
				Str(logging.KeySuspicious, "true").
				Msgf("received invalid sync response from %x: %v", originID[:], valid)
			e.metrics.InboundMessageDropped(metrics.EngineSynchronization, metrics.MessageSyncResponse)
			return nil
		}
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

// TODO: implement spam reporting similar to validateSyncRequestForALSP
func (e *Engine) validateBatchRequestForALSP(channel channels.Channel, id flow.Identifier, batchRequest *messages.BatchRequest) (*alsp.MisbehaviorReport, bool, error) {
	return nil, true, nil
}

// TODO: implement spam reporting similar to validateSyncRequestForALSP
func (e *Engine) validateBlockResponseForALSP(channel channels.Channel, id flow.Identifier, blockResponse *messages.BlockResponse) (*alsp.MisbehaviorReport, bool, error) {
	return nil, true, nil
}

// validateRangeRequestForALSP checks if a range request should be reported as a misbehavior.
// It returns a misbehavior report and a boolean indicating whether validation passed, as well as an error.
// Returns an error that is assumed to be irrecoverable because of internal processes that didn't allow validation to complete.
// Returns true if the range request is valid and should not be reported as misbehavior.
// Returns false if either a) the range request is invalid or b) the range request is valid but should be reported as misbehavior anyway (due to probabilities) or c) an error is encountered.
func (e *Engine) validateRangeRequestForALSP(originID flow.Identifier, rangeRequest *messages.RangeRequest) (*alsp.MisbehaviorReport, bool, error) {
	// Generate a random integer between 1 and spamProbabilityMultiplier (exclusive)
	n, err := rand.Uint32n(spamProbabilityMultiplier)

	if err != nil {
		return nil, false, fmt.Errorf("failed to generate random number from %x: %w", originID[:], err)
	}

	// check if range request is valid
	if rangeRequest.ToHeight < rangeRequest.FromHeight {
		e.log.Warn().
			Hex("origin_id", logging.ID(originID)).
			Str(logging.KeySuspicious, "true").
			Str("reason", alsp.InvalidMessage.String()).
			Msgf("received invalid range request from height %d is not less than the to height %d, creating ALSP report", rangeRequest.FromHeight, rangeRequest.ToHeight)
		report, err := alsp.NewMisbehaviorReport(originID, alsp.InvalidMessage)

		if err != nil {
			// failing to create the misbehavior report is unlikely. If an error is encountered while
			// creating the misbehavior report it indicates a bug and processing can not proceed.
			return nil, false, fmt.Errorf("failed to create misbehavior report (invalid range request) from %x: %w", originID[:], err)
		}
		// failed validation check and should be reported as misbehavior
		return report, false, nil
	}

	// to avoid creating a misbehavior report for every range request received, use a probabilistic approach.
	// The higher the range request and base probability, the higher the probability of creating a misbehavior report.

	// rangeRequestProb is calculated as follows:
	// rangeRequestBaseProb * ((rangeRequest.ToHeight-rangeRequest.FromHeight) + 1) / synccore.DefaultConfig().MaxSize
	// Example 1 (small range) if the range request is for 10 blocks and rangeRequestBaseProb is 0.01, then the probability of
	// creating a misbehavior report is:
	// rangeRequestBaseProb * (10+1) / synccore.DefaultConfig().MaxSize
	// = 0.01 * 11 / 64 = 0.00171875 = 0.171875%
	// Example 2 (large range) if the range request is for 1000 blocks and rangeRequestBaseProb is 0.01, then the probability of
	// creating a misbehavior report is:
	// rangeRequestBaseProb * (1000+1) / synccore.DefaultConfig().MaxSize
	// = 0.01 * 1001 / 64 = 0.15640625 = 15.640625%
	rangeRequestProb := e.spamDetectionConfig.rangeRequestBaseProb * (float32(rangeRequest.ToHeight-rangeRequest.FromHeight) + 1) / float32(synccore.DefaultConfig().MaxSize)
	if float32(n) < rangeRequestProb*spamProbabilityMultiplier {
		// create a misbehavior report
		e.log.Warn().
			Hex("origin_id", logging.ID(originID)).
			Str(logging.KeySuspicious, "true").
			Str("reason", alsp.ResourceIntensiveRequest.String()).
			Msgf("from height %d to height %d, creating probabilistic ALSP report", rangeRequest.FromHeight, rangeRequest.ToHeight)
		report, err := alsp.NewMisbehaviorReport(originID, alsp.ResourceIntensiveRequest)

		if err != nil {
			// failing to create the misbehavior report is unlikely. If an error is encountered while
			// creating the misbehavior report it indicates a bug and processing can not proceed.
			return nil, false, fmt.Errorf("failed to create misbehavior report from %x: %w", originID[:], err)
		}
		// failed validation check and should be reported as misbehavior
		return report, false, nil
	}

	// passed all validation checks with no misbehavior detected
	return nil, true, nil
}

// validateSyncRequestForALSP checks if a sync request should be reported as a misbehavior.
// It returns a misbehavior report and a boolean indicating whether validation passed, as well as an error.
// Returns an error that is assumed to be irrecoverable because of internal processes that didn't allow validation to complete.
// Returns true if passed validation.
// Returns false if either a) failed validation (due to probabilities) or b) an error is encountered.
func (e *Engine) validateSyncRequestForALSP(originID flow.Identifier) (*alsp.MisbehaviorReport, bool, error) {
	// Generate a random integer between 1 and spamProbabilityMultiplier (exclusive)
	n, err := rand.Uint32n(spamProbabilityMultiplier)

	if err != nil {
		return nil, false, fmt.Errorf("failed to generate random number from %x: %w", originID[:], err)
	}

	// to avoid creating a misbehavior report for every sync request received, use a probabilistic approach.
	// Create a report with a probability of spamDetectionConfig.syncRequestProb
	if float32(n) < e.spamDetectionConfig.syncRequestProb*spamProbabilityMultiplier {

		// create misbehavior report
		e.log.Warn().
			Hex("origin_id", logging.ID(originID)).
			Str(logging.KeySuspicious, "true").
			Str("reason", alsp.ResourceIntensiveRequest.String()).
			Msg("creating probabilistic ALSP report")

		report, err := alsp.NewMisbehaviorReport(originID, alsp.ResourceIntensiveRequest)

		if err != nil {
			// failing to create the misbehavior report is unlikely. If an error is encountered while
			// creating the misbehavior report it indicates a bug and processing can not proceed.
			return nil, false, fmt.Errorf("failed to create misbehavior report from %x: %w", originID[:], err)
		}
		return report, false, nil
	}

	// passed all validation checks with no misbehavior detected
	return nil, true, nil
}

// TODO: implement spam reporting similar to validateSyncRequestForALSP
func (e *Engine) validateSyncResponseForALSP(channel channels.Channel, id flow.Identifier, syncResponse *messages.SyncResponse) (*alsp.MisbehaviorReport, bool, error) {
	return nil, true, nil
}
