package sealing

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type Event struct {
	OriginID flow.Identifier
	Msg      interface{}
}

// defaultApprovalQueueCapacity maximum capacity of approvals queue
const defaultApprovalQueueCapacity = 10000

// defaultApprovalResponseQueueCapacity maximum capacity of approval requests queue
const defaultApprovalResponseQueueCapacity = 10000

// defaultFinalizationEventsQueueCapacity maximum capacity of finalization events
const defaultFinalizationEventsQueueCapacity = 1000

// defaultSealingEngineWorkers number of workers to dispatch events for sealing core
const defaultSealingEngineWorkers = 8

type (
	EventSink chan *Event // Channel to push pending events
)

// Engine is a wrapper for approval processing `Core` which implements logic for
// queuing and filtering network messages which later will be processed by sealing engine.
// Purpose of this struct is to provide an efficient way how to consume messages from network layer and pass
// them to `Core`. Engine runs 2 separate gorourtines that perform pre-processing and consuming messages by Core.
type Engine struct {
	unit                       *engine.Unit
	core                       consensus.SealingCore
	log                        zerolog.Logger
	me                         module.Local
	headers                    storage.Headers
	payloads                   storage.Payloads
	cacheMetrics               module.MempoolMetrics
	engineMetrics              module.EngineMetrics
	pendingApprovals           engine.MessageStore
	pendingRequestedApprovals  engine.MessageStore
	pendingFinalizationEvents  *fifoqueue.FifoQueue
	pendingIncorporatedResults *fifoqueue.FifoQueue
	notifier                   engine.Notifier
	messageHandler             *engine.MessageHandler
	rootHeader                 *flow.Header
}

// NewEngine constructs new `Engine` which runs on it's own unit.
func NewEngine(log zerolog.Logger,
	tracer module.Tracer,
	conMetrics module.ConsensusMetrics,
	engineMetrics module.EngineMetrics,
	mempool module.MempoolMetrics,
	sealingTracker consensus.SealingTracker,
	net module.Network,
	me module.Local,
	headers storage.Headers,
	payloads storage.Payloads,
	state protocol.State,
	sealsDB storage.Seals,
	assigner module.ChunkAssigner,
	verifier module.Verifier,
	sealsMempool mempool.IncorporatedResultSeals,
	options Config,
) (*Engine, error) {
	rootHeader, err := state.Params().Root()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root block: %w", err)
	}

	unit := engine.NewUnit()
	e := &Engine{
		unit:          unit,
		log:           log.With().Str("engine", "sealing.Engine").Logger(),
		me:            me,
		engineMetrics: engineMetrics,
		cacheMetrics:  mempool,
		headers:       headers,
		payloads:      payloads,
		rootHeader:    rootHeader,
	}

	err = e.setupTrustedInboundQueues()
	if err != nil {
		return nil, fmt.Errorf("initialization of inbound queues for trusted inputs failed: %w", err)
	}

	err = e.setupMessageHandler(options.RequiredApprovalsForSealConstruction)
	if err != nil {
		return nil, fmt.Errorf("could not initialize message handler for untrusted inputs: %w", err)
	}

	// register engine with the approval provider
	_, err = net.Register(engine.ReceiveApprovals, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for approvals: %w", err)
	}

	// register engine to the channel for requesting missing approvals
	approvalConduit, err := net.Register(engine.RequestApprovalsByChunk, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for requesting approvals: %w", err)
	}

	core, err := NewCore(log, tracer, conMetrics, sealingTracker, unit, headers, state, sealsDB, assigner, verifier, sealsMempool, approvalConduit, options)
	if err != nil {
		return nil, fmt.Errorf("failed to init sealing engine: %w", err)
	}

	err = core.RepopulateAssignmentCollectorTree(payloads)
	if err != nil {
		return nil, fmt.Errorf("could not repopulate assignment collectors tree: %w", err)
	}
	e.core = core

	return e, nil
}

// setupTrustedInboundQueues initializes inbound queues for TRUSTED INPUTS (from other components within the
// consensus node). We deliberately separate the queues for trusted inputs from the MessageHandler, which
// handles external, untrusted inputs. This reduces the attack surface, as it makes it impossible for an external
// attacker to feed values into the inbound channels for trusted inputs, even in the presence of bugs in
// the networking layer or message handler
func (e *Engine) setupTrustedInboundQueues() error {
	var err error
	e.pendingFinalizationEvents, err = fifoqueue.NewFifoQueue(fifoqueue.WithCapacity(defaultFinalizationEventsQueueCapacity))
	if err != nil {
		return fmt.Errorf("failed to create queue for finalization events: %w", err)
	}
	e.pendingIncorporatedResults, err = fifoqueue.NewFifoQueue()
	if err != nil {
		return fmt.Errorf("failed to create queue for incorproated results: %w", err)
	}
	return nil
}

// setupMessageHandler initializes the inbound queues and the MessageHandler for UNTRUSTED INPUTS.
func (e *Engine) setupMessageHandler(requiredApprovalsForSealConstruction uint) error {
	// FIFO queue for broadcasted approvals
	pendingApprovalsQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultApprovalQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { e.cacheMetrics.MempoolEntries(metrics.ResourceApprovalQueue, uint(len)) }),
	)
	if err != nil {
		return fmt.Errorf("failed to create queue for inbound approvals: %w", err)
	}
	e.pendingApprovals = &engine.FifoMessageStore{
		FifoQueue: pendingApprovalsQueue,
	}

	// FiFo queue for requested approvals
	pendingRequestedApprovalsQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultApprovalResponseQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { e.cacheMetrics.MempoolEntries(metrics.ResourceApprovalResponseQueue, uint(len)) }),
	)
	if err != nil {
		return fmt.Errorf("failed to create queue for requested approvals: %w", err)
	}
	e.pendingRequestedApprovals = &engine.FifoMessageStore{
		FifoQueue: pendingRequestedApprovalsQueue,
	}

	e.notifier = engine.NewNotifier()
	// define message queueing behaviour
	e.messageHandler = engine.NewMessageHandler(
		e.log,
		e.notifier,
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*flow.ResultApproval)
				if ok {
					e.engineMetrics.MessageReceived(metrics.EngineSealing, metrics.MessageResultApproval)
				}
				return ok
			},
			Map: func(msg *engine.Message) (*engine.Message, bool) {
				if requiredApprovalsForSealConstruction < 1 {
					// if we don't require approvals to construct a seal, don't even process approvals.
					return nil, false
				}

				return msg, true
			},
			Store: e.pendingApprovals,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*messages.ApprovalResponse)
				if ok {
					e.engineMetrics.MessageReceived(metrics.EngineSealing, metrics.MessageResultApproval)
				}
				return ok
			},
			Map: func(msg *engine.Message) (*engine.Message, bool) {
				if requiredApprovalsForSealConstruction < 1 {
					// if we don't require approvals to construct a seal, don't even process approvals.
					return nil, false
				}

				approval := msg.Payload.(*messages.ApprovalResponse).Approval
				return &engine.Message{
					OriginID: msg.OriginID,
					Payload:  &approval,
				}, true
			},
			Store: e.pendingRequestedApprovals,
		},
	)

	return nil
}

// Process sends event into channel with pending events. Generally speaking shouldn't lock for too long.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.messageHandler.Process(originID, event)
}

// processAvailableMessages is processor of pending events which drives events from networking layer to business logic in `Core`.
// Effectively consumes messages from networking layer and dispatches them into corresponding sinks which are connected with `Core`.
func (e *Engine) processAvailableMessages() error {
	for {
		select {
		case <-e.unit.Quit():
			return nil
		default:
		}

		event, ok := e.pendingFinalizationEvents.Pop()
		if ok {
			finalizedBlockID := event.(flow.Identifier)
			err := e.core.ProcessFinalizedBlock(finalizedBlockID)
			if err != nil {
				return fmt.Errorf("could not process finalized block %v: %w", finalizedBlockID, err)
			}
			continue
		}

		event, ok = e.pendingIncorporatedResults.Pop()
		if ok {
			err := e.processIncorporatedResult(event.(*flow.ExecutionResult))
			if err != nil {
				return fmt.Errorf("could not process incorporated result: %w", err)
			}
			continue
		}

		// TODO prioritization
		// eg: msg := engine.SelectNextMessage()
		msg, ok := e.pendingRequestedApprovals.Get()
		if !ok {
			msg, ok = e.pendingApprovals.Get()
		}
		if ok {
			err := e.onApproval(msg.OriginID, msg.Payload.(*flow.ResultApproval))
			if err != nil {
				return fmt.Errorf("could not process result approval: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

func (e *Engine) loop() {
	notifier := e.notifier.Channel()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-notifier:
			err := e.processAvailableMessages()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing queued message")
			}
		}
	}
}

// processIncorporatedResult is a function that creates incorporated result and submits it for processing
// to sealing core. In phase 2, incorporated result is incorporated at same block that is being executed.
// This will be changed in phase 3.
func (e *Engine) processIncorporatedResult(result *flow.ExecutionResult) error {
	// TODO: change this when migrating to sealing & verification phase 3.
	// Incorporated result is created this way only for phase 2.
	incorporatedResult := flow.NewIncorporatedResult(result.BlockID, result)
	err := e.core.ProcessIncorporatedResult(incorporatedResult)
	e.engineMetrics.MessageHandled(metrics.EngineSealing, metrics.MessageExecutionReceipt)
	return err
}

func (e *Engine) onApproval(originID flow.Identifier, approval *flow.ResultApproval) error {
	// don't process approval if originID is mismatched
	if originID != approval.Body.ApproverID {
		return nil
	}

	err := e.core.ProcessApproval(approval)
	e.engineMetrics.MessageHandled(metrics.EngineSealing, metrics.MessageResultApproval)
	if err != nil {
		return fmt.Errorf("fatal internal error in sealing core logic")
	}
	return nil
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	err := e.Process(originID, event)
	if err != nil {
		engine.LogError(e.log, err)
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	// launch as many workers as we need
	for i := 0; i < defaultSealingEngineWorkers; i++ {
		e.unit.Launch(e.loop)
	}
	return e.unit.Ready()
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
//  (1) Informs sealing.Core about finalization of respective block.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(finalizedBlockID flow.Identifier) {
	e.pendingFinalizationEvents.Push(finalizedBlockID)
	e.notifier.Notify()
}

// OnBlockIncorporated implements `OnBlockIncorporated` from the `hotstuff.FinalizationConsumer`
//  (1) Processes all execution results that were incorporated in parent block payload.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnBlockIncorporated(incorporatedBlockID flow.Identifier) {
	e.unit.Launch(func() {
		// In order to process a block within the sealing engine, we need the block's source of
		// randomness (to compute the chunk assignment). The source of randomness can be taken from _any_
		// QC for the block. We know that we have such a QC, once a valid child block is incorporated.
		// Vice-versa, once a block is incorporated, we know that _its parent_ has a valid child, i.e.
		// the parent's source of randomness is now know.

		incorporatedBlock, err := e.headers.ByBlockID(incorporatedBlockID)
		if err != nil {
			e.log.Fatal().Err(err).Msgf("could not retrieve header for block %v", incorporatedBlockID)
		}

		e.log.Info().Msgf("processing incorporated block %v at height %d", incorporatedBlockID, incorporatedBlock.Height)

		// we are interested in blocks with height strictly larger than root block
		if incorporatedBlock.Height <= e.rootHeader.Height {
			return
		}

		payload, err := e.payloads.ByBlockID(incorporatedBlock.ParentID)
		if err != nil {
			e.log.Fatal().Err(err).Msgf("could not retrieve payload for block %v", incorporatedBlock.ParentID)
		}

		for _, result := range payload.Results {
			added := e.pendingIncorporatedResults.Push(result)
			if !added {
				// Not being able to queue an incorporated result is a fatal edge case. It might happen, if the
				// queue capacity is depleted. However, we cannot dropped the incorporated result, because there
				// is no way that an incorporated result can be re-added later once dropped.
				e.log.Fatal().Msg("failed to queue incorporated result")
			}
		}
		e.notifier.Notify()
	})
}
