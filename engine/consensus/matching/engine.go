package matching

import (
	"fmt"

	"github.com/ef-ds/deque"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
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

// defaultReceiptQueueCapacity maximum capacity of receipts queue
const defaultReceiptQueueCapacity = 10000

// defaultApprovalQueueCapacity maximum capacity of approvals queue
const defaultApprovalQueueCapacity = 10000

// defaultApprovalResponseQueueCapacity maximum capacity of approval requests queue
const defaultApprovalResponseQueueCapacity = 10000

type (
	EventProvider <-chan *Event // Channel to get pending events that are ready to be processed
	EventSink     chan<- *Event // Channel to push pending events
)

// Engine is a wrapper for matching `Core` which implements logic for
// queuing and filtering network messages which later will be processed by matching engine.
// Purpose of this struct is to provide an efficient way how to consume messages from network layer and pass
// them to `Core`.
type Engine struct {
	unit                                 *engine.Unit
	log                                  zerolog.Logger
	me                                   module.Local
	core                                 *Core
	engineMetrics                        module.EngineMetrics
	receiptSink                          EventSink
	approvalSink                         EventSink
	approvalResponseSink                 EventSink
	pendingReceipts                      deque.Deque
	pendingApprovals                     deque.Deque
	pendingApprovalResponses             deque.Deque
	pendingEventSink                     chan *Event
	requiredApprovalsForSealConstruction uint
}

// NewEngine constructs new `EngineEngine` which runs on it's own unit.
func NewEngine(log zerolog.Logger,
	engineMetrics module.EngineMetrics,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	conMetrics module.ConsensusMetrics,
	net module.Network,
	state protocol.State,
	me module.Local,
	receiptRequester module.Requester,
	receiptsDB storage.ExecutionReceipts,
	headersDB storage.Headers,
	indexDB storage.Index,
	incorporatedResults mempool.IncorporatedResults,
	receipts mempool.ExecutionTree,
	approvals mempool.Approvals,
	seals mempool.IncorporatedResultSeals,
	assigner module.ChunkAssigner,
	validator module.ReceiptValidator,
	requiredApprovalsForSealConstruction uint,
	emergencySealingActive bool) (*Engine, error) {
	// create channels that will be used to feed data to matching core.
	receiptsChannel := make(chan *Event)
	approvalsChannel := make(chan *Event)
	approvalResponsesChannel := make(chan *Event)
	e := &Engine{
		unit:                                 engine.NewUnit(),
		log:                                  log,
		me:                                   me,
		core:                                 nil,
		engineMetrics:                        engineMetrics,
		receiptSink:                          receiptsChannel,
		approvalSink:                         approvalsChannel,
		approvalResponseSink:                 approvalResponsesChannel,
		pendingEventSink:                     make(chan *Event),
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
	}

	// register engine with the receipt provider
	_, err := net.Register(engine.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
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

	e.core, err = NewCore(log, engineMetrics, tracer, mempool, conMetrics, state, me, receiptRequester, receiptsDB, headersDB,
		indexDB, incorporatedResults, receipts, approvals, seals, assigner, validator,
		requiredApprovalsForSealConstruction, emergencySealingActive, receiptsChannel, approvalsChannel, approvalResponsesChannel, approvalConduit)
	if err != nil {
		return nil, fmt.Errorf("failed to init matching engine: %w", err)
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
	fetchEvent := func(queue *deque.Deque, sink EventSink) (*Event, EventSink) {
		event, ok := queue.PopFront()
		if !ok {
			return nil, nil
		}
		return event.(*Event), sink
	}

	for {
		pendingReceipt, receiptSink := fetchEvent(&e.pendingReceipts, e.receiptSink)
		pendingApproval, approvalSink := fetchEvent(&e.pendingApprovals, e.approvalSink)
		pendingApprovalResponse, approvalResponseSink := fetchEvent(&e.pendingApprovalResponses, e.approvalResponseSink)
		select {
		case event := <-e.pendingEventSink:
			e.processPendingEvent(event)
		case receiptSink <- pendingReceipt:
			continue
		case approvalSink <- pendingApproval:
			continue
		case approvalResponseSink <- pendingApprovalResponse:
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
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		if e.pendingReceipts.Len() < defaultReceiptQueueCapacity {
			e.pendingReceipts.PushBack(event)
		}
	case *flow.ResultApproval:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return
		}
		if e.pendingApprovals.Len() < defaultApprovalQueueCapacity {
			e.pendingApprovals.PushBack(event)
		}
	case *messages.ApprovalResponse:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return
		}
		if e.pendingApprovalResponses.Len() < defaultApprovalResponseQueueCapacity {
			e.pendingApprovalResponses.PushBack(event)
		}
	}
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
	started := make(chan struct{})
	e.unit.Launch(func() {
		close(started)
		e.processEvents()
	})
	<-started
	return e.core.Ready()
}

func (e *Engine) Done() <-chan struct{} {
	<-e.unit.Done()
	return e.core.Done()
}
