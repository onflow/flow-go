package sealing

import (
	"fmt"
	"runtime"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	sealing "github.com/onflow/flow-go/engine/consensus"
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

type (
	EventSink chan *Event // Channel to push pending events
)

// Engine is a wrapper for approval processing `Core` which implements logic for
// queuing and filtering network messages which later will be processed by sealing engine.
// Purpose of this struct is to provide an efficient way how to consume messages from network layer and pass
// them to `Core`. Engine runs 2 separate gorourtines that perform pre-processing and consuming messages by Core.
type Engine struct {
	unit                                 *engine.Unit
	core                                 sealing.SealingCore
	workerPool                           *workerpool.WorkerPool
	log                                  zerolog.Logger
	me                                   module.Local
	headers                              storage.Headers
	payloads                             storage.Payloads
	cacheMetrics                         module.MempoolMetrics
	engineMetrics                        module.EngineMetrics
	pendingApprovals                     engine.MessageStore
	pendingRequestedApprovals            engine.MessageStore
	messageHandler                       *engine.MessageHandler
	requiredApprovalsForSealConstruction uint
	rootHeader                           *flow.Header
}

// NewEngine constructs new `Engine` which runs on it's own unit.
func NewEngine(log zerolog.Logger,
	tracer module.Tracer,
	conMetrics module.ConsensusMetrics,
	engineMetrics module.EngineMetrics,
	mempool module.MempoolMetrics,
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

	hardwareConcurrency := runtime.NumCPU()

	rootHeader, err := state.Params().Root()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root block: %w", err)
	}

	e := &Engine{
		unit:                                 engine.NewUnit(),
		log:                                  log.With().Str("engine", "sealing.Engine").Logger(),
		me:                                   me,
		engineMetrics:                        engineMetrics,
		cacheMetrics:                         mempool,
		headers:                              headers,
		payloads:                             payloads,
		workerPool:                           workerpool.New(hardwareConcurrency),
		requiredApprovalsForSealConstruction: options.RequiredApprovalsForSealConstruction,
		rootHeader:                           rootHeader,
	}

	err = e.setupMessageHandler()
	if err != nil {
		return nil, fmt.Errorf("could not initialize message handler: %w", err)
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

	e.core, err = NewCore(log, tracer, conMetrics, headers, state, sealsDB, assigner, verifier, sealsMempool, approvalConduit, options)
	if err != nil {
		return nil, fmt.Errorf("failed to init sealing engine: %w", err)
	}

	return e, nil
}

func (e *Engine) setupMessageHandler() error {
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

	// define message queueing behaviour
	e.messageHandler = engine.NewMessageHandler(
		e.log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*flow.ResultApproval)
				if ok {
					e.engineMetrics.MessageReceived(metrics.EngineSealing, metrics.MessageResultApproval)
				}
				return ok
			},
			Map: func(msg *engine.Message) (*engine.Message, bool) {
				if e.requiredApprovalsForSealConstruction < 1 {
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
				if e.requiredApprovalsForSealConstruction < 1 {
					// if we don't require approvals to construct a seal, don't even process approvals.
					return nil, false
				}

				return &engine.Message{
					OriginID: msg.OriginID,
					Payload:  msg.Payload.(*messages.ApprovalResponse).Approval,
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
		// TODO prioritization
		// eg: msg := engine.SelectNextMessage()
		msg, ok := e.pendingRequestedApprovals.Get()
		if !ok {
			msg, ok = e.pendingApprovals.Get()
		}

		if ok {
			e.onApproval(msg.OriginID, msg.Payload.(*flow.ResultApproval))
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

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

// processIncorporatedResult is a function that creates incorporated result and submits it for processing
// to sealing core. In phase 2, incorporated result is incorporated at same block that is being executed.
// This will be changed in phase 3.
func (e *Engine) processIncorporatedResult(result *flow.ExecutionResult) {
	e.workerPool.Submit(func() {
		incorporatedResult := flow.NewIncorporatedResult(result.BlockID, result)
		err := e.core.ProcessIncorporatedResult(incorporatedResult)
		e.engineMetrics.MessageHandled(metrics.EngineSealing, metrics.MessageExecutionReceipt)

		if err != nil {
			e.log.Fatal().Err(err).Msgf("fatal internal error in sealing core logic")
		}
	})
}

func (e *Engine) onApproval(originID flow.Identifier, approval *flow.ResultApproval) {
	// don't process approval if originID is mismatched
	if originID != approval.Body.ApproverID {
		return
	}

	e.workerPool.Submit(func() {
		err := e.core.ProcessApproval(approval)
		e.engineMetrics.MessageHandled(metrics.EngineSealing, metrics.MessageResultApproval)
		if err != nil {
			e.log.Fatal().Err(err).Msgf("fatal internal error in sealing core logic")
		}
	})
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
	e.unit.Launch(e.loop)
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
	e.log.Info().Msgf("processing finalized block: %v", finalizedBlockID)

	e.workerPool.Submit(func() {
		err := e.core.ProcessFinalizedBlock(finalizedBlockID)
		if err != nil {
			e.log.Fatal().Err(err).Msgf("critical sealing error when processing finalized block %v", finalizedBlockID)
		}
	})
}

// OnBlockIncorporated implements `OnBlockIncorporated` from the `hotstuff.FinalizationConsumer`
//  (1) Processes all execution results that were incorporated in parent block payload.
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnBlockIncorporated(incorporatedBlockID flow.Identifier) {
	e.log.Info().Msgf("processing incorporated block: %v", incorporatedBlockID)

	e.workerPool.Submit(func() {
		// We can't process incorporated block because of how sealing engine handles assignments we need to
		// make sure that block has children. Instead we will process parent block

		incorporatedBlock, err := e.headers.ByBlockID(incorporatedBlockID)
		if err != nil {
			e.log.Fatal().Err(err).Msgf("could not retrieve header for block %v", incorporatedBlockID)
		}

		// we are interested in blocks with height strictly larger than root block
		if incorporatedBlock.Height <= e.rootHeader.Height {
			return
		}

		payload, err := e.payloads.ByBlockID(incorporatedBlock.ParentID)
		if err != nil {
			e.log.Fatal().Err(err).Msgf("could not retrieve payload for block %v", incorporatedBlock.ParentID)
		}

		for _, result := range payload.Results {
			e.processIncorporatedResult(result)
		}
	})
}
