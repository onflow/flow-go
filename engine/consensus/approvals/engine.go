package approvals

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/fifoqueue"
)

type Event struct {
	OriginID flow.Identifier
	Msg      interface{}
}

// defaultReceiptQueueCapacity maximum capacity of receipts queue
const defaultReceiptQueueCapacity = 10000

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
	core                                 consensus.ResultApprovalProcessor
	workerPool                           *workerpool.WorkerPool
	log                                  zerolog.Logger
	me                                   module.Local
	payloads                             storage.Payloads
	cacheMetrics                         module.MempoolMetrics
	engineMetrics                        module.EngineMetrics
	receiptSink                          EventSink
	approvalSink                         EventSink
	requestedApprovalSink                EventSink
	pendingReceipts                      *fifoqueue.FifoQueue
	pendingApprovals                     *fifoqueue.FifoQueue
	pendingRequestedApprovals            *fifoqueue.FifoQueue
	pendingEventSink                     EventSink
	requiredApprovalsForSealConstruction uint
}

// NewEngine constructs new `Engine` which runs on it's own unit.
func NewEngine(log zerolog.Logger,
	engineMetrics module.EngineMetrics,
	core consensus.ResultApprovalProcessor,
	mempool module.MempoolMetrics,
	net module.Network,
	me module.Local,
	requiredApprovalsForSealConstruction uint,
) (*Engine, error) {

	hardwareConcurrency := runtime.NumCPU()

	e := &Engine{
		unit:                                 engine.NewUnit(),
		log:                                  log,
		me:                                   me,
		core:                                 core,
		engineMetrics:                        engineMetrics,
		cacheMetrics:                         mempool,
		receiptSink:                          make(EventSink),
		approvalSink:                         make(EventSink),
		requestedApprovalSink:                make(EventSink),
		pendingEventSink:                     make(EventSink),
		workerPool:                           workerpool.New(hardwareConcurrency),
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
	}

	// FIFO queue for inbound receipts
	var err error
	e.pendingReceipts, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultReceiptQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { mempool.MempoolEntries(metrics.ResourceReceiptQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound receipts: %w", err)
	}

	// FIFO queue for broadcasted approvals
	e.pendingApprovals, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultApprovalQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { mempool.MempoolEntries(metrics.ResourceApprovalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound approvals: %w", err)
	}

	// FiFo queue for requested approvals
	e.pendingRequestedApprovals, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultApprovalResponseQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { mempool.MempoolEntries(metrics.ResourceApprovalResponseQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for requested approvals: %w", err)
	}

	// register engine with the receipt provider
	_, err = net.Register(engine.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	// register engine with the approval provider
	_, err = net.Register(engine.ReceiveApprovals, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for approvals: %w", err)
	}

	return e, nil
}

// Process sends event into channel with pending events. Generally speaking shouldn't lock for too long.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	e.pendingEventSink <- &Event{
		OriginID: originID,
		Msg:      event,
	}
	return nil
}

// processEvents is processor of pending events which drives events from networking layer to business logic in `Core`.
// Effectively consumes messages from networking layer and dispatches them into corresponding sinks which are connected with `Core`.
// Should be run as a separate goroutine.
func (e *Engine) processEvents() {
	// takes pending event from one of the queues
	// nil sink means nothing to send, this prevents blocking on select
	fetchEvent := func() (*Event, EventSink, *fifoqueue.FifoQueue) {
		if val, ok := e.pendingReceipts.Front(); ok {
			return val.(*Event), e.receiptSink, e.pendingReceipts
		}
		if val, ok := e.pendingRequestedApprovals.Front(); ok {
			return val.(*Event), e.requestedApprovalSink, e.pendingRequestedApprovals
		}
		if val, ok := e.pendingApprovals.Front(); ok {
			return val.(*Event), e.approvalSink, e.pendingApprovals
		}
		return nil, nil, nil
	}

	for {
		pendingEvent, sink, fifo := fetchEvent()
		select {
		case event := <-e.pendingEventSink:
			e.processPendingEvent(event)
		case sink <- pendingEvent:
			fifo.Pop()
			continue
		case <-e.unit.Quit():
			return
		}
	}
}

// processPendingEvent saves pending event in corresponding queue for further processing by `Core`.
// While this function runs in separate goroutine it shouldn't do heavy processing to maintain efficient data polling/pushing.
func (e *Engine) processPendingEvent(event *Event) {
	switch event.Msg.(type) {
	case *flow.ExecutionReceipt:
		e.engineMetrics.MessageReceived(metrics.EngineSealing, metrics.MessageExecutionReceipt)
		e.pendingReceipts.Push(event)
	case *flow.ResultApproval:
		e.engineMetrics.MessageReceived(metrics.EngineSealing, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return
		}
		e.pendingApprovals.Push(event)
	case *messages.ApprovalResponse:
		e.engineMetrics.MessageReceived(metrics.EngineSealing, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return
		}
		e.pendingRequestedApprovals.Push(event)
	}
}

// consumeEvents consumes events that are ready to be processed.
func (e *Engine) consumeEvents() {
	for {
		select {
		case event := <-e.receiptSink:
			e.processIncorporatedResult(&event.Msg.(*flow.ExecutionReceipt).ExecutionResult)
		case event := <-e.approvalSink:
			e.onApproval(event.OriginID, event.Msg.(*flow.ResultApproval))
		case event := <-e.requestedApprovalSink:
			e.onApproval(event.OriginID, &event.Msg.(*messages.ApprovalResponse).Approval)
		case <-e.unit.Quit():
			return
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

// HandleReceipt pipes explicitly requested receipts to the process function.
// Receipts can come from this function or the receipt provider setup in the
// engine constructor.
func (e *Engine) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {
	e.log.Debug().Msg("received receipt from requester engine")

	err := e.Process(originID, receipt)
	if err != nil {
		e.log.Error().Err(err).Hex("origin", originID[:]).Msg("could not process receipt")
	}
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	var wg sync.WaitGroup
	wg.Add(2)
	e.unit.Launch(func() {
		wg.Done()
		e.processEvents()
	})
	e.unit.Launch(func() {
		wg.Done()
		e.consumeEvents()
	})
	return e.unit.Ready(func() {
		wg.Wait()
	})
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// OnFinalizedBlock process finalization event from hotstuff. Processes all results that were submitted in payload.
func (e *Engine) OnFinalizedBlock(finalizedBlockID flow.Identifier) {
	payload, err := e.payloads.ByBlockID(finalizedBlockID)
	if err != nil {
		e.log.Fatal().Err(err).Msgf("could not retrieve payload for block %v", finalizedBlockID)
	}

	for _, result := range payload.Results {
		e.processIncorporatedResult(result)
	}
}
