package sealing

import (
	"fmt"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
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

// defaultSealingEngineWorkers number of workers to dispatch events for sealing core
const defaultSealingEngineWorkers = 8

// defaultAssignmentCollectorsWorkerPoolCapacity is the default number of workers that is available for worker pool which is used
// by assignment collector state machine to do transitions
const defaultAssignmentCollectorsWorkerPoolCapacity = 4

// defaultIncorporatedBlockQueueCapacity maximum capacity of block incorporated events queue
const defaultIncorporatedBlockQueueCapacity = 1000

type (
	EventSink chan *Event // Channel to push pending events
)

// Engine is a wrapper for approval processing `Core` which implements logic for
// queuing and filtering network messages which later will be processed by sealing engine.
// Purpose of this struct is to provide an efficient way how to consume messages from network layer and pass
// them to `Core`. Engine runs 2 separate gorourtines that perform pre-processing and consuming messages by Core.
type Engine struct {
	unit                       *engine.Unit
	workerPool                 *workerpool.WorkerPool
	core                       consensus.SealingCore
	log                        zerolog.Logger
	me                         module.Local
	headers                    storage.Headers
	results                    storage.ExecutionResults
	index                      storage.Index
	state                      protocol.State
	cacheMetrics               module.MempoolMetrics
	engineMetrics              module.EngineMetrics
	pendingApprovals           engine.MessageStore
	pendingRequestedApprovals  engine.MessageStore
	pendingIncorporatedResults *fifoqueue.FifoQueue
	pendingIncorporatedBlocks  *fifoqueue.FifoQueue
	inboundEventsNotifier      engine.Notifier
	finalizationEventsNotifier engine.Notifier
	blockIncorporatedNotifier  engine.Notifier
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
	net network.Network,
	me module.Local,
	headers storage.Headers,
	payloads storage.Payloads,
	results storage.ExecutionResults,
	index storage.Index,
	state protocol.State,
	sealsDB storage.Seals,
	assigner module.ChunkAssigner,
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
		workerPool:    workerpool.New(defaultAssignmentCollectorsWorkerPoolCapacity),
		log:           log.With().Str("engine", "sealing.Engine").Logger(),
		me:            me,
		state:         state,
		engineMetrics: engineMetrics,
		cacheMetrics:  mempool,
		headers:       headers,
		results:       results,
		index:         index,
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

	signatureHasher := crypto.NewBLSKMAC(encoding.ResultApprovalTag)
	core, err := NewCore(log, e.workerPool, tracer, conMetrics, sealingTracker, unit, headers, state, sealsDB, assigner, signatureHasher, sealsMempool, approvalConduit, options)
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
	e.finalizationEventsNotifier = engine.NewNotifier()
	e.blockIncorporatedNotifier = engine.NewNotifier()
	var err error
	e.pendingIncorporatedResults, err = fifoqueue.NewFifoQueue()
	if err != nil {
		return fmt.Errorf("failed to create queue for incorproated results: %w", err)
	}
	e.pendingIncorporatedBlocks, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultIncorporatedBlockQueueCapacity))
	if err != nil {
		return fmt.Errorf("failed to create queue for incorproated blocks: %w", err)
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

	e.inboundEventsNotifier = engine.NewNotifier()
	// define message queueing behaviour
	e.messageHandler = engine.NewMessageHandler(
		e.log,
		e.inboundEventsNotifier,
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
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
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

// processAvailableMessages is processor of pending events which drives events from networking layer to business logic in `Core`.
// Effectively consumes messages from networking layer and dispatches them into corresponding sinks which are connected with `Core`.
func (e *Engine) processAvailableMessages() error {
	for {
		select {
		case <-e.unit.Quit():
			return nil
		default:
		}

		event, ok := e.pendingIncorporatedResults.Pop()
		if ok {
			e.log.Debug().Msg("got new incorporated result")

			err := e.processIncorporatedResult(event.(*flow.IncorporatedResult))
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
			e.log.Debug().Msg("got new result approval")

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

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop() {
	finalizationNotifier := e.finalizationEventsNotifier.Channel()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-finalizationNotifier:
			finalized, err := e.state.Final().Head()
			if err != nil {
				e.log.Fatal().Err(err).Msg("could not retrieve last finalized block")
			}
			err = e.core.ProcessFinalizedBlock(finalized.ID())
			if err != nil {
				e.log.Fatal().Err(err).Msgf("could not process finalized block %v", finalized.ID())
			}
		}
	}
}

// blockIncorporatedEventsProcessingLoop is a separate goroutine for processing block incorporated events
func (e *Engine) blockIncorporatedEventsProcessingLoop() {
	c := e.blockIncorporatedNotifier.Channel()

	for {
		select {
		case <-e.unit.Quit():
			return
		case <-c:
			err := e.processBlockIncorporatedEvents()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing block incorporated queued message")
			}
		}
	}
}

func (e *Engine) loop() {
	notifier := e.inboundEventsNotifier.Channel()
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
func (e *Engine) processIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error {
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
	err := e.ProcessLocal(event)
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
	return e.messageHandler.Process(e.me.NodeID(), event)
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	// launch as many workers as we need
	for i := 0; i < defaultSealingEngineWorkers; i++ {
		e.unit.Launch(e.loop)
	}
	e.unit.Launch(e.finalizationProcessingLoop)
	e.unit.Launch(e.blockIncorporatedEventsProcessingLoop)
	return e.unit.Ready()
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		e.workerPool.StopWait()
	})
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
//  (1) Informs sealing.Core about finalization of respective block.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(*model.Block) {
	e.finalizationEventsNotifier.Notify()
}

// OnBlockIncorporated implements `OnBlockIncorporated` from the `hotstuff.FinalizationConsumer`
//  (1) Processes all execution results that were incorporated in parent block payload.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnBlockIncorporated(incorporatedBlock *model.Block) {
	e.pendingIncorporatedBlocks.Push(incorporatedBlock.BlockID)
	e.blockIncorporatedNotifier.Notify()
}

// processIncorporatedBlock selects receipts that were included into incorporated block and submits them
// for further processing to sealing core.
func (e *Engine) processIncorporatedBlock(incorporatedBlockID flow.Identifier) error {
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
		return nil
	}

	index, err := e.index.ByBlockID(incorporatedBlock.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve payload index for block %v", incorporatedBlock.ParentID)
	}

	for _, resultID := range index.ResultIDs {
		result, err := e.results.ByID(resultID)
		if err != nil {
			return fmt.Errorf("could not retrieve receipt incorporated in block %v: %w", incorporatedBlock.ParentID, err)
		}

		incorporatedResult := flow.NewIncorporatedResult(incorporatedBlock.ParentID, result)
		added := e.pendingIncorporatedResults.Push(incorporatedResult)
		if !added {
			// Not being able to queue an incorporated result is a fatal edge case. It might happen, if the
			// queue capacity is depleted. However, we cannot dropped the incorporated result, because there
			// is no way that an incorporated result can be re-added later once dropped.
			return fmt.Errorf("failed to queue incorporated result")
		}
	}
	e.inboundEventsNotifier.Notify()
	return nil
}

// processBlockIncorporatedEvents performs processing of block incorporated hot stuff events
func (e *Engine) processBlockIncorporatedEvents() error {
	for {
		select {
		case <-e.unit.Quit():
			return nil
		default:
		}

		msg, ok := e.pendingIncorporatedBlocks.Pop()
		if ok {
			err := e.processIncorporatedBlock(msg.(flow.Identifier))
			if err != nil {
				return fmt.Errorf("could not process incorporated block: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}
