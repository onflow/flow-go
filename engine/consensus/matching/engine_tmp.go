package matching

import (
	"fmt"
	"sync"
	"time"

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
	EventSink chan *Event // Channel to push pending events
)

// EngineTmp is a wrapper for matching `Engine` which implements logic for
// queuing and filtering network messages which later will be processed by matching engine.
// Purpose of this struct is to provide an efficient way how to consume messages from network layer and pass
// them to `Engine`. EngineTmp runs 2 separate gorourtines that perform pre-processing and consuming messages by Engine.
type EngineTmp struct {
	unit                                 *engine.Unit
	log                                  zerolog.Logger
	me                                   module.Local
	core                                 *Engine
	cacheMetrics                         module.MempoolMetrics
	engineMetrics                        module.EngineMetrics
	receiptSink                          EventSink
	approvalSink                         EventSink
	approvalResponseSink                 EventSink
	pendingReceipts                      deque.Deque
	pendingApprovals                     deque.Deque
	pendingApprovalResponses             deque.Deque
	pendingEventSink                     EventSink
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
	emergencySealingActive bool) (*EngineTmp, error) {
	e := &EngineTmp{
		unit:                                 engine.NewUnit(),
		log:                                  log,
		me:                                   me,
		core:                                 nil,
		engineMetrics:                        engineMetrics,
		cacheMetrics:                         mempool,
		receiptSink:                          make(EventSink),
		approvalSink:                         make(EventSink),
		approvalResponseSink:                 make(EventSink),
		pendingEventSink:                     make(EventSink),
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
		requiredApprovalsForSealConstruction, emergencySealingActive, approvalConduit)
	if err != nil {
		return nil, fmt.Errorf("failed to init matching engine: %w", err)
	}

	return e, nil
}

// Process sends event into channel with pending events. Generally speaking shouldn't lock for too long.
func (e *EngineTmp) Process(originID flow.Identifier, event interface{}) error {
	e.pendingEventSink <- &Event{
		OriginID: originID,
		Msg:      event,
	}
	return nil
}

// processEvents is processor of pending events which drives events from networking layer to business logic in `Engine`.
// Effectively consumes messages from networking layer and dispatches them into corresponding sinks which are connected with `Engine`.
// Should be run as a separate goroutine.
func (e *EngineTmp) processEvents() {
	// takes pending event from one of the queues
	// nil sink means nothing to send, this prevents blocking on select
	fetchEvent := func() (*Event, EventSink, *deque.Deque) {
		if val, ok := e.pendingReceipts.Front(); ok {
			return val.(*Event), e.receiptSink, &e.pendingReceipts
		}
		if val, ok := e.pendingApprovals.Front(); ok {
			return val.(*Event), e.approvalSink, &e.pendingApprovals
		}
		if val, ok := e.pendingApprovalResponses.Front(); ok {
			return val.(*Event), e.approvalResponseSink, &e.pendingApprovalResponses
		}
		return nil, nil, nil
	}

	for {
		pendingEvent, sink, fifo := fetchEvent()
		select {
		case event := <-e.pendingEventSink:
			e.processPendingEvent(event)
		case sink <- pendingEvent:
			fifo.PopFront()
			continue
		case <-e.unit.Quit():
			return
		}
	}
}

// processPendingEvent saves pending event in corresponding queue for further processing by `Engine`.
// While this function runs in separate goroutine it shouldn't do heavy processing to maintain efficient data polling/pushing.
func (e *EngineTmp) processPendingEvent(event *Event) {
	switch event.Msg.(type) {
	case *flow.ExecutionReceipt:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		if e.pendingReceipts.Len() < defaultReceiptQueueCapacity {
			e.pendingReceipts.PushBack(event)
		}
		e.cacheMetrics.MempoolEntries(metrics.ResourceReceiptQueue, uint(e.pendingReceipts.Len()))
	case *flow.ResultApproval:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return
		}
		if e.pendingApprovals.Len() < defaultApprovalQueueCapacity {
			e.pendingApprovals.PushBack(event)
		}
		e.cacheMetrics.MempoolEntries(metrics.ResourceApprovalQueue, uint(e.pendingApprovals.Len()))
	case *messages.ApprovalResponse:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return
		}
		if e.pendingApprovalResponses.Len() < defaultApprovalResponseQueueCapacity {
			e.pendingApprovalResponses.PushBack(event)
		}
		e.cacheMetrics.MempoolEntries(metrics.ResourceApprovalResponseQueue,
			uint(e.pendingApprovalResponses.Len()))
	}
}

// consumeEvents consumes events that are ready to be processed.
func (e *EngineTmp) consumeEvents() {
	// Context:
	// We expect a lot more Approvals compared to blocks or receipts. However, the level of
	// information only changes significantly with new blocks or new receipts.
	// We used to kick off the sealing check after every approval and receipt. In cases where
	// the sealing check takes a lot more time than processing the actual messages (which we
	// assume for the current implementation), we incur a large overhead as we check a lot
	// of conditions, which only change with new blocks or new receipts.
	// TEMPORARY FIX: to avoid sealing checks to monopolize the engine and delay processing
	// of receipts and approvals. Specifically, we schedule sealing checks every 2 seconds.
	checkSealingTicker := make(chan struct{})
	defer close(checkSealingTicker)
	e.unit.LaunchPeriodically(func() {
		checkSealingTicker <- struct{}{}
	}, 2*time.Second, 120*time.Second)

	for {
		select {
		case event := <-e.receiptSink:
			e.consumeSingleEvent(event)
		case event := <-e.approvalSink:
			e.consumeSingleEvent(event)
		case event := <-e.approvalResponseSink:
			e.consumeSingleEvent(event)
		case <-checkSealingTicker:
			e.core.checkSealing()
		case <-e.unit.Quit():
			return
		}
	}
}

// consumeSingleEvent processes single event for the propagation engine on the consensus node.
func (e *EngineTmp) consumeSingleEvent(pendingEvent *Event) {
	var err error
	switch event := pendingEvent.Msg.(type) {
	case *flow.ExecutionReceipt:
		defer e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		err = e.core.onReceipt(pendingEvent.OriginID, event)
	case *flow.ResultApproval:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		defer e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageResultApproval)
		err = e.core.onApproval(pendingEvent.OriginID, event)
	case *messages.ApprovalResponse:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		defer e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageResultApproval)
		err = e.core.onApproval(pendingEvent.OriginID, &event.Approval)
	default:
		err = fmt.Errorf("invalid event type (%T)", pendingEvent.Msg)
	}
	if err != nil {
		// TODO: we probably want to check the type of the error here
		// * If the message was invalid (e.g. NewInvalidInputError), which is expected as part
		//   of normal operations in an untrusted environment, we can just log an error.
		// * However, if we receive an error indicating internal node failure or state
		//   internal state corruption, we should probably crash the node.
		e.log.Error().Err(err).Hex("origin", pendingEvent.OriginID[:]).Msgf("could not process event")
	}
}

// SubmitLocal submits an event originating on the local node.
func (e *EngineTmp) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *EngineTmp) Submit(originID flow.Identifier, event interface{}) {
	err := e.Process(originID, event)
	if err != nil {
		engine.LogError(e.log, err)
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *EngineTmp) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// HandleReceipt pipes explicitly requested receipts to the process function.
// Receipts can come from this function or the receipt provider setup in the
// engine constructor.
func (e *EngineTmp) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {
	e.log.Debug().Msg("received receipt from requester engine")

	err := e.Process(originID, receipt)
	if err != nil {
		e.log.Error().Err(err).Hex("origin", originID[:]).Msg("could not process receipt")
	}
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (e *EngineTmp) Ready() <-chan struct{} {
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

func (e *EngineTmp) Done() <-chan struct{} {
	return e.unit.Done()
}
