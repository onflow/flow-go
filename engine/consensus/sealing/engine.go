package sealing

import (
	"fmt"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
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

// defaultIncorporatedBlockQueueCapacity maximum capacity for queuing incorporated blocks
// Caution: We cannot drop incorporated blocks, as there is no way that results included in the block
// can be re-added later once dropped. Missing any incorporated result can undermine sealing liveness!
// Therefore, the queue capacity should be large _and_ there should be logic for crashing the node
// in case queueing an incorporated block fails.
const defaultIncorporatedBlockQueueCapacity = 10000

// defaultIncorporatedResultQueueCapacity maximum capacity for queuing incorporated results
// Caution: We cannot drop incorporated results, as there is no way that an incorporated result
// can be re-added later once dropped. Missing incorporated results can undermine sealing liveness!
// Therefore, the queue capacity should be large _and_ there should be logic for crashing the node
// in case queueing an incorporated result fails.
const defaultIncorporatedResultQueueCapacity = 80000

type (
	EventSink chan *Event // Channel to push pending events
)

// Engine is a wrapper for approval processing `Core` which implements logic for
// queuing and filtering network messages which later will be processed by sealing engine.
// Purpose of this struct is to provide an efficient way to consume messages from the network layer and pass
// them to `Core`. Engine runs multiple workers for pre-processing messages and executing `sealing.Core` business logic.
type Engine struct {
	component.Component
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

// NewEngine constructs a new Sealing Engine which runs on its own component.
func NewEngine(log zerolog.Logger,
	tracer module.Tracer,
	conMetrics module.ConsensusMetrics,
	engineMetrics module.EngineMetrics,
	mempool module.MempoolMetrics,
	sealingTracker consensus.SealingTracker,
	net network.EngineRegistry,
	me module.Local,
	headers storage.Headers,
	payloads storage.Payloads,
	results storage.ExecutionResults,
	index storage.Index,
	state protocol.State,
	sealsDB storage.Seals,
	assigner module.ChunkAssigner,
	sealsMempool mempool.IncorporatedResultSeals,
	requiredApprovalsForSealConstructionGetter module.SealingConfigsGetter,
) (*Engine, error) {
	rootHeader := state.Params().FinalizedRoot()

	e := &Engine{
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

	err := e.setupTrustedInboundQueues()
	if err != nil {
		return nil, fmt.Errorf("initialization of inbound queues for trusted inputs failed: %w", err)
	}

	err = e.setupMessageHandler(requiredApprovalsForSealConstructionGetter)
	if err != nil {
		return nil, fmt.Errorf("could not initialize message handler for untrusted inputs: %w", err)
	}

	e.Component = e.buildComponentManager()

	// register engine with the approval provider
	_, err = net.Register(channels.ReceiveApprovals, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for approvals: %w", err)
	}

	// register engine to the channel for requesting missing approvals
	approvalConduit, err := net.Register(channels.RequestApprovalsByChunk, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for requesting approvals: %w", err)
	}

	signatureHasher := msig.NewBLSHasher(msig.ResultApprovalTag)
	core, err := NewCore(log, e.workerPool, tracer, conMetrics, sealingTracker, headers, state, sealsDB, assigner, signatureHasher, sealsMempool, approvalConduit, requiredApprovalsForSealConstructionGetter)
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

// buildComponentManager creates the component manager with the necessary workers.
// It must only be called during initialization of the sealing engine, and the only
// reason it is factored out from NewEngine is so that it can be used in tests.
func (e *Engine) buildComponentManager() *component.ComponentManager {
	builder := component.NewComponentManagerBuilder()
	for i := 0; i < defaultSealingEngineWorkers; i++ {
		builder.AddWorker(e.loop)
	}
	builder.AddWorker(e.finalizationProcessingLoop)
	builder.AddWorker(e.blockIncorporatedEventsProcessingLoop)
	builder.AddWorker(e.waitUntilWorkersFinish)
	return builder.Build()
}

// waitUntilWorkersFinish ensures that the Sealing Engine only finishes shutting down
// once the workerPool used by the Sealing Core has been shut down (after waiting
// for any pending tasks to complete).
func (e *Engine) waitUntilWorkersFinish(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	<-ctx.Done()
	// After receiving shutdown signal, wait for the workerPool
	e.workerPool.StopWait()
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
	e.pendingIncorporatedResults, err = fifoqueue.NewFifoQueue(defaultIncorporatedResultQueueCapacity)
	if err != nil {
		return fmt.Errorf("failed to create queue for incorporated results: %w", err)
	}
	e.pendingIncorporatedBlocks, err = fifoqueue.NewFifoQueue(defaultIncorporatedBlockQueueCapacity)
	if err != nil {
		return fmt.Errorf("failed to create queue for incorporated blocks: %w", err)
	}
	return nil
}

// setupMessageHandler initializes the inbound queues and the MessageHandler for UNTRUSTED INPUTS.
func (e *Engine) setupMessageHandler(getSealingConfigs module.SealingConfigsGetter) error {
	// FIFO queue for broadcasted approvals
	pendingApprovalsQueue, err := fifoqueue.NewFifoQueue(
		defaultApprovalQueueCapacity,
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
		defaultApprovalResponseQueueCapacity,
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
				if getSealingConfigs.RequireApprovalsForSealConstructionDynamicValue() < 1 {
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
				if getSealingConfigs.RequireApprovalsForSealConstructionDynamicValue() < 1 {
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
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	err := e.messageHandler.Process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
			return nil
		}
		// An unexpected exception should never happen here, because the `messageHandler` only puts the events into the
		// respective queues depending on their type or returns an `IncompatibleInputTypeError` for events with unknown type.
		// We cannot return the error here, because the networking layer calling `Process`  will just log that error and
		// continue on a best-effort basis, which is not safe in case of an unexpected exception.
		e.log.Fatal().Err(err).Msg("unexpected error while processing engine message")
	}
	return nil
}

// processAvailableMessages is processor of pending events which drives events from networking layer to business logic in `Core`.
// Effectively consumes messages from networking layer and dispatches them into corresponding sinks which are connected with `Core`.
// No errors expected during normal operations.
func (e *Engine) processAvailableMessages(ctx irrecoverable.SignalerContext) error {
	for {
		select {
		case <-ctx.Done():
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

// finalizationProcessingLoop contains the logic for processing of block finalization events.
// This method is intended to be executed by a single worker goroutine.
func (e *Engine) finalizationProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	finalizationNotifier := e.finalizationEventsNotifier.Channel()
	ready()
	for {
		select {
		case <-ctx.Done():
			return
		case <-finalizationNotifier:
			finalized, err := e.state.Final().Head()
			if err != nil {
				ctx.Throw(fmt.Errorf("could not retrieve last finalized block: %w", err))
			}
			err = e.core.ProcessFinalizedBlock(finalized.ID())
			if err != nil {
				ctx.Throw(fmt.Errorf("could not process finalized block %v: %w", finalized.ID(), err))
			}
		}
	}
}

// blockIncorporatedEventsProcessingLoop contains the logic for processing block incorporated events.
// This method is intended to be executed by a single worker goroutine.
func (e *Engine) blockIncorporatedEventsProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	c := e.blockIncorporatedNotifier.Channel()
	ready()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c:
			err := e.processBlockIncorporatedEvents(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("internal error processing block incorporated queued message: %w", err))
			}
		}
	}
}

// loop contains the logic for processing incorporated results and result approvals via sealing.Core's
// business logic. This method is intended to be executed by multiple loop worker goroutines concurrently.
func (e *Engine) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	notifier := e.inboundEventsNotifier.Channel()
	ready()
	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			err := e.processAvailableMessages(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("internal error processing queued message: %w", err))
			}
		}
	}
}

// processIncorporatedResult is a function that creates incorporated result and submits it for processing
// to sealing core. In phase 2, incorporated result is incorporated at same block that is being executed.
// This will be changed in phase 3.
// No errors expected during normal operations.
func (e *Engine) processIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error {
	err := e.core.ProcessIncorporatedResult(incorporatedResult)
	e.engineMetrics.MessageHandled(metrics.EngineSealing, metrics.MessageExecutionReceipt)
	return err
}

// onApproval checks that the result approval is valid before forwarding it to the Core for processing in a blocking way.
// No errors expected during normal operations.
func (e *Engine) onApproval(originID flow.Identifier, approval *flow.ResultApproval) error {
	// don't process (silently ignore) approval if originID is mismatched
	if originID != approval.Body.ApproverID {
		e.log.Warn().Bool(logging.KeyProtocolViolation, true).
			Msgf("result approval generated by node %v received from different originID %v", approval.Body.ApproverID, originID)
		return nil
	}

	err := e.core.ProcessApproval(approval)
	e.engineMetrics.MessageHandled(metrics.EngineSealing, metrics.MessageResultApproval)
	if err != nil {
		return irrecoverable.NewExceptionf("fatal internal error in sealing core logic: %w", err)
	}
	return nil
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// It informs sealing.Core about finalization of respective block.
//
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(*model.Block) {
	e.finalizationEventsNotifier.Notify()
}

// OnBlockIncorporated implements `OnBlockIncorporated` from the `hotstuff.FinalizationConsumer`
// It processes all execution results that were incorporated in parent block payload.
//
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnBlockIncorporated(incorporatedBlock *model.Block) {
	added := e.pendingIncorporatedBlocks.Push(incorporatedBlock.BlockID)
	if !added {
		// Not being able to queue an incorporated block is a fatal edge case. It might happen, if the
		// queue capacity is depleted. However, we cannot drop incorporated blocks, because there
		// is no way that any contained incorporated result would be re-added later once dropped.
		e.log.Fatal().Msgf("failed to queue incorporated block %v", incorporatedBlock.BlockID)
	}
	e.blockIncorporatedNotifier.Notify()
}

// processIncorporatedBlock selects receipts that were included into incorporated block and submits them
// for further processing to sealing core. No errors expected during normal operations.
func (e *Engine) processIncorporatedBlock(incorporatedBlockID flow.Identifier) error {
	// In order to process a block within the sealing engine, we need the block's source of
	// randomness (to compute the chunk assignment). The source of randomness can be taken from _any_
	// QC for the block. We know that we have such a QC, once a valid child block is incorporated.
	// Vice-versa, once a block is incorporated, we know that _its parent_ has a valid child, i.e.
	// the parent's source of randomness is now know.

	incorporatedBlock, err := e.headers.ByBlockID(incorporatedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve header for block %v", incorporatedBlockID)
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
			// queue capacity is depleted. However, we cannot drop incorporated results, because there
			// is no way that an incorporated result can be re-added later once dropped.
			return fmt.Errorf("failed to queue incorporated result")
		}
	}
	e.inboundEventsNotifier.Notify()
	return nil
}

// processBlockIncorporatedEvents performs processing of block incorporated hot stuff events
// No errors expected during normal operations.
func (e *Engine) processBlockIncorporatedEvents(ctx irrecoverable.SignalerContext) error {
	for {
		select {
		case <-ctx.Done():
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
