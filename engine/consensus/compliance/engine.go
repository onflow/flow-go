package compliance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// defaultRangeResponseQueueCapacity maximum capacity of inbound queue for `messages.BlockResponse`s
const defaultRangeResponseQueueCapacity = 100

// defaultBlockQueueCapacity maximum capacity of inbound queue for `messages.BlockProposal`s
const defaultBlockQueueCapacity = 10000

// defaultVoteQueueCapacity maximum capacity of inbound queue for `messages.BlockVote`s
const defaultVoteQueueCapacity = 1000

// defaultTimeoutObjectsQueueCapacity maximum capacity of inbound queue for `messages.TimeoutObject`s
const defaultTimeoutObjectsQueueCapacity = 1000

// Engine is a wrapper around `compliance.Core`. The Engine queues inbound messages, relevant
// node-internal notifications, and manages the worker routines processing the inbound events,
// and forwards outbound messages to the networking layer.
// `compliance.Core` implements the actual compliance logic.
type Engine struct {
	unit                       *engine.Unit
	lm                         *lifecycle.LifecycleManager
	log                        zerolog.Logger
	mempool                    module.MempoolMetrics
	metrics                    module.EngineMetrics
	me                         module.Local
	headers                    storage.Headers
	payloads                   storage.Payloads
	tracer                     module.Tracer
	state                      protocol.State
	prov                       network.Engine
	core                       *Core
	pendingBlocks              engine.MessageStore
	pendingRangeResponses      engine.MessageStore
	pendingVotes               engine.MessageStore
	pendingTimeouts            engine.MessageStore
	messageHandler             *engine.MessageHandler
	finalizedView              counters.StrictMonotonousCounter
	finalizationEventsNotifier engine.Notifier
	con                        network.Conduit
	stopHotstuff               context.CancelFunc
}

var _ hotstuff.Communicator = (*Engine)(nil)

func NewEngine(
	log zerolog.Logger,
	net network.Network,
	me module.Local,
	prov network.Engine,
	core *Core,
) (*Engine, error) {
	// Inbound FIFO queue for `messages.BlockResponse`s
	rangeResponseQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultRangeResponseQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { core.mempool.MempoolEntries(metrics.ResourceBlockResponseQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for block-sync responses: %w", err)
	}
	pendingRangeResponses := &engine.FifoMessageStore{
		FifoQueue: rangeResponseQueue,
	}

	// Inbound FIFO queue for `messages.BlockProposal`s
	blocksQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultBlockQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { core.mempool.MempoolEntries(metrics.ResourceBlockProposalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound block proposals: %w", err)
	}
	pendingBlocks := &engine.FifoMessageStore{FifoQueue: blocksQueue}

	// Inbound FIFO queue for `messages.BlockVote`s
	votesQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultVoteQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { core.mempool.MempoolEntries(metrics.ResourceBlockVoteQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound votes: %w", err)
	}
	pendingVotes := &engine.FifoMessageStore{FifoQueue: votesQueue}

	// Inbound FIFO queue for `messages.TimeoutObject`s
	// TODO(active-pacemaker): update metrics
	timeoutObjectsQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultTimeoutObjectsQueueCapacity))
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound timeout objects: %w", err)
	}
	pendingTimeouts := &engine.FifoMessageStore{FifoQueue: timeoutObjectsQueue}

	// define message queueing behaviour
	handler := engine.NewMessageHandler(
		log.With().Str("compliance", "engine").Logger(),
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.BlockResponse)
				if ok {
					core.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockResponse)
				}
				return ok
			},
			Store: pendingRangeResponses,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.BlockProposal)
				if ok {
					core.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockProposal)
				}
				return ok
			},
			Store: pendingBlocks,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*events.SyncedBlock)
				if ok {
					core.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageSyncedBlock)
				}
				return ok
			},
			Map: func(msg *engine.Message) (*engine.Message, bool) {
				syncedBlock := msg.Payload.(*events.SyncedBlock)
				msg = &engine.Message{
					OriginID: msg.OriginID,
					Payload: &messages.BlockProposal{
						Payload: syncedBlock.Block.Payload,
						Header:  syncedBlock.Block.Header,
					},
				}
				return msg, true
			},
			Store: pendingBlocks,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.BlockVote)
				if ok {
					core.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockVote)
				}
				return ok
			},
			Store: pendingVotes,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.TimeoutObject)
				// TODO(active-pacemaker): update metrics
				//if ok {
				//core.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockVote)
				//}
				return ok
			},
			Store: pendingTimeouts,
		},
	)

	eng := &Engine{
		unit:                       engine.NewUnit(),
		lm:                         lifecycle.NewLifecycleManager(),
		log:                        log.With().Str("compliance", "engine").Logger(),
		me:                         me,
		mempool:                    core.mempool,
		metrics:                    core.metrics,
		headers:                    core.headers,
		payloads:                   core.payloads,
		pendingRangeResponses:      pendingRangeResponses,
		pendingBlocks:              pendingBlocks,
		pendingVotes:               pendingVotes,
		pendingTimeouts:            pendingTimeouts,
		state:                      core.state,
		tracer:                     core.tracer,
		prov:                       prov,
		core:                       core,
		messageHandler:             handler,
		finalizationEventsNotifier: engine.NewNotifier(),
	}

	// register the core with the network layer and store the conduit
	eng.con, err = net.Register(channels.ConsensusCommittee, eng)
	if err != nil {
		return nil, fmt.Errorf("could not register core: %w", err)
	}

	return eng, nil
}

// WithConsensus adds the consensus algorithm to the engine. This must be
// called before the engine can start.
func (e *Engine) WithConsensus(hot module.HotStuff) *Engine {
	e.core.hotstuff = hot
	return e
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.core.hotstuff == nil {
		panic("must initialize compliance engine with hotstuff engine")
	}
	e.lm.OnStart(func() {
		e.unit.Launch(e.loop)
		e.unit.Launch(e.finalizationProcessingLoop)

		ctx, cancel := context.WithCancel(context.Background())
		signalerCtx, hotstuffErrChan := irrecoverable.WithSignaler(ctx)
		e.stopHotstuff = cancel

		// TODO: this workaround for handling fatal HotStuff errors is required only
		//  because this engine and epochmgr do not use the Component pattern yet
		e.unit.Launch(func() {
			e.handleHotStuffError(hotstuffErrChan)
		})

		e.core.hotstuff.Start(signalerCtx)
		// wait for request handler to startup

		<-e.core.hotstuff.Ready()
	})
	return e.lm.Started()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	e.lm.OnStop(func() {
		e.log.Info().Msg("shutting down hotstuff eventloop")
		e.stopHotstuff()
		<-e.core.hotstuff.Done()
		e.log.Info().Msg("all components have been shut down")
		<-e.unit.Done()
	})
	return e.lm.Stopped()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	err := e.ProcessLocal(event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel channels.Channel, originID flow.Identifier, event interface{}) {
	err := e.Process(channel, originID, event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.messageHandler.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	err := e.messageHandler.Process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
			return nil
		}
		return fmt.Errorf("unexpected error while processing engine message: %w", err)
	}
	return nil
}

// loop implements the processing of inbound messages. Only returns when Engine is terminated.
func (e *Engine) loop() {
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-e.messageHandler.GetNotifier():
			err := e.processAvailableMessages()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing queued message")
			}
		}
	}
}

// processAvailableMessages processes any available messages from the inbound queues.
// Only returns when all inbound queues are empty (or the engine is terminated).
// No errors expected during normal operations.
func (e *Engine) processAvailableMessages() error {
	for {
		select {
		case <-e.unit.Quit():
			return nil
		default:
		}

		msg, ok := e.pendingRangeResponses.Get()
		if ok {
			blockResponse := msg.Payload.(*messages.BlockResponse)
			for _, block := range blockResponse.Blocks {
				// process each block and indicate it's from a range of blocks
				err := e.core.OnBlockProposal(msg.OriginID, &messages.BlockProposal{
					Header:  block.Header,
					Payload: block.Payload,
				}, true)
				if err != nil {
					return fmt.Errorf("could not process synced block proposal: %w", err)
				}
			}
			continue
		}

		msg, ok = e.pendingBlocks.Get()
		if ok {
			err := e.core.OnBlockProposal(msg.OriginID, msg.Payload.(*messages.BlockProposal), true)
			if err != nil {
				return fmt.Errorf("could not handle block proposal: %w", err)
			}
			continue
		}

		msg, ok = e.pendingVotes.Get()
		if ok {
			err := e.core.OnBlockVote(msg.OriginID, msg.Payload.(*messages.BlockVote))
			if err != nil {
				return fmt.Errorf("could not handle block vote: %w", err)
			}
			continue
		}

		msg, ok = e.pendingTimeouts.Get()
		if ok {
			err := e.core.OnTimeoutObject(msg.OriginID, msg.Payload.(*messages.TimeoutObject))
			if err != nil {
				return fmt.Errorf("could not handle timeout object: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) error {
	log := e.log.With().
		Hex("block_id", blockID[:]).
		Uint64("block_view", view).
		Hex("recipient_id", recipientID[:]).
		Logger()

	log.Info().Msg("processing vote transmission request from hotstuff")

	// build the vote message
	vote := &messages.BlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sigData,
	}

	// TODO: this is a hot-fix to mitigate the effects of the following Unicast call blocking occasionally
	e.unit.Launch(func() {
		// send the vote the desired recipient
		err := e.con.Unicast(vote, recipientID)
		if err != nil {
			log.Warn().Err(err).Msg("could not send vote")
			return
		}
		e.metrics.MessageSent(metrics.EngineCompliance, metrics.MessageBlockVote)
		log.Info().Msg("block vote transmitted")
	})

	return nil
}

// BroadcastTimeout submits a timeout object to all consensus nodes.
// No errors are expected during normal operation.
func (e *Engine) BroadcastTimeout(timeout *model.TimeoutObject) error {
	logContext := e.log.With().
		Uint64("timeout_newest_qc_view", timeout.NewestQC.View).
		Hex("timeout_newest_qc_block_id", timeout.NewestQC.BlockID[:]).
		Uint64("timeout_view", timeout.View)

	if timeout.LastViewTC != nil {
		logContext.
			Uint64("last_view_tc_view", timeout.LastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", timeout.LastViewTC.NewestQC.View)
	}
	log := logContext.Logger()

	log.Info().Msg("processing timeout broadcast request from hotstuff")
	e.unit.Launch(func() {
		// Retrieve all consensus nodes (excluding myself).
		// CAUTION: We must include also nodes with weight zero, because otherwise
		//          TCs might not be constructed at epoch switchover.
		// Note: retrieving the final state requires a time-intensive database read.
		//       Therefore, we execute this in a separate routine, because
		//       `BroadcastTimeout` is directly called by the consensus core logic.
		recipients, err := e.state.Final().Identities(filter.And(
			filter.HasRole(flow.RoleConsensus),
			filter.Not(filter.HasNodeID(e.me.NodeID())),
		))
		if err != nil {
			e.log.Fatal().Err(err).Msg("could not get consensus recipients for broadcasting timeout")
		}

		// create the proposal message for the collection
		msg := &messages.TimeoutObject{
			View:       timeout.View,
			NewestQC:   timeout.NewestQC,
			LastViewTC: timeout.LastViewTC,
			SigData:    timeout.SigData,
		}

		err = e.con.Publish(msg, recipients.NodeIDs()...)
		if errors.Is(err, network.EmptyTargetList) {
			return
		}
		if err != nil {
			log.Error().Err(err).Msg("could not broadcast timeout")
			return
		}
		log.Info().Msg("consensus timeout broadcast")

		// TODO(active-pacemaker): update metrics
		//e.metrics.MessageSent(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
		//e.core.collectionMetrics.ClusterBlockProposed(block)
	})

	return nil
}

// BroadcastProposalWithDelay will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.
// No errors are expected during normal operation.
func (e *Engine) BroadcastProposalWithDelay(header *flow.Header, delay time.Duration) error {
	// first, check that we are the proposer of the block
	if header.ProposerID != e.me.NodeID() {
		return fmt.Errorf("cannot broadcast proposal with non-local proposer (%x)", header.ProposerID)
	}

	// get the parent of the block
	parent, err := e.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// fill in the fields that can't be populated by HotStuff
	header.ChainID = parent.ChainID
	header.Height = parent.Height + 1

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not retrieve payload for proposal: %w", err)
	}

	log := e.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Int("gaurantees_count", len(payload.Guarantees)).
		Int("seals_count", len(payload.Seals)).
		Int("receipts_count", len(payload.Receipts)).
		Time("timestamp", header.Timestamp).
		Hex("signers", header.ParentVoterIndices).
		Dur("delay", delay).
		Logger()

	log.Debug().Msg("processing proposal broadcast request from hotstuff")

	e.unit.LaunchAfter(delay, func() {
		// Retrieve all consensus nodes (excluding myself).
		// CAUTION: We must include also nodes with weight zero, because otherwise
		//          new consensus nodes for the next epoch are left out.
		// Note: retrieving the final state requires a time-intensive database read.
		//       Therefore, we execute this in a separate routine, because
		//       `BroadcastTimeout` is directly called by the consensus core logic.
		recipients, err := e.state.AtBlockID(header.ParentID).Identities(filter.And(
			filter.HasRole(flow.RoleConsensus),
			filter.Not(filter.HasNodeID(e.me.NodeID())),
		))
		if err != nil {
			e.log.Fatal().Err(err).Msg("could not get consensus recipient for broadcasting proposal")
		}

		// forward proposal to node's local consensus instance
		e.core.hotstuff.SubmitProposal(header, parent.View) // non-blocking

		// NOTE: some fields are not needed for the message
		// - proposer ID is conveyed over the network message
		// - the payload hash is deduced from the payload
		proposal := &messages.BlockProposal{
			Header:  header,
			Payload: payload,
		}

		// broadcast the proposal to consensus nodes
		err = e.con.Publish(proposal, recipients.NodeIDs()...)
		if errors.Is(err, network.EmptyTargetList) {
			return
		}
		if err != nil {
			log.Error().Err(err).Msg("could not send proposal message")
		}

		e.metrics.MessageSent(metrics.EngineCompliance, metrics.MessageBlockProposal)

		log.Info().Msg("block proposal broadcasted")

		// submit the proposal to the provider engine to forward it to other
		// node roles
		e.prov.SubmitLocal(proposal)
	})

	return nil
}

// BroadcastProposal will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.
// No errors are expected during normal operation.
func (e *Engine) BroadcastProposal(header *flow.Header) error {
	return e.BroadcastProposalWithDelay(header, 0)
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// It informs sealing.Core about finalization of respective block.
//
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	if e.finalizedView.Set(block.View) {
		e.finalizationEventsNotifier.Notify()
	}
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop() {
	finalizationNotifier := e.finalizationEventsNotifier.Channel()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-finalizationNotifier:
			e.core.ProcessFinalizedView(e.finalizedView.Value())
		}
	}
}

// handleHotStuffError accepts the error channel from the HotStuff component and
// crashes the node if any error is detected.
//
//	 TODO: this function should be removed in favour of refactoring this engine and
//		 	 the epochmgr engine to use the Component pattern, so that irrecoverable errors
//			 can be bubbled all the way to the node scaffold
func (e *Engine) handleHotStuffError(hotstuffErrs <-chan error) {
	for {
		select {
		case <-e.unit.Quit():
			return
		case err := <-hotstuffErrs:
			if err != nil {
				e.log.Fatal().Err(err).Msg("encountered fatal error in HotStuff")
			}
		}
	}
}
