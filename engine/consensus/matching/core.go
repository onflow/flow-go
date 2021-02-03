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

// EventProvider is an interface which provides polling mechanism for
// events that are ready to be processedengine/consensus/matching/core_test.go
type (
	EventProvider <-chan *Event
	EventSink     chan<- *Event
)

// Core2 is a wrapper for matching `Engine` which implements logic for
// queuing and filtering network messages which later will be processed by matching engine.
// Purpose of this struct is to provide an efficient way how to consume messages from network layer and pass
// them to `Engine`.
type Core2 struct {
	unit                     *engine.Unit
	log                      zerolog.Logger
	me                       module.Local
	engine                   *Engine
	engineMetrics            module.EngineMetrics
	receiptSink              EventSink
	approvalSink             EventSink
	approvalResponseSink     EventSink
	pendingReceipts          deque.Deque
	pendingApprovals         deque.Deque
	pendingApprovalResponses deque.Deque
	pendingEventSink         chan *Event
}

// NewEngine constructs new `Core2` which runs on it's own unit.
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
	emergencySealingActive bool) (*Core2, error) {
	// create channels that will be used to feed data to matching core.
	receiptsChannel := make(chan *Event)
	approvalsChannel := make(chan *Event)
	approvalResponsesChannel := make(chan *Event)
	c := &Core2{
		unit:                 engine.NewUnit(),
		log:                  log,
		me:                   me,
		engine:               nil,
		engineMetrics:        engineMetrics,
		receiptSink:          receiptsChannel,
		approvalSink:         approvalsChannel,
		approvalResponseSink: approvalResponsesChannel,
		pendingEventSink:     make(chan *Event),
	}

	// register engine with the receipt provider
	_, err := net.Register(engine.ReceiveReceipts, c)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	// register engine with the approval provider
	_, err = net.Register(engine.ReceiveApprovals, c)
	if err != nil {
		return nil, fmt.Errorf("could not register for approvals: %w", err)
	}

	// register engine to the channel for requesting missing approvals
	approvalConduit, err := net.Register(engine.RequestApprovalsByChunk, c)
	if err != nil {
		return nil, fmt.Errorf("could not register for requesting approvals: %w", err)
	}

	c.engine, err = NewCore(log, engineMetrics, tracer, mempool, conMetrics, state, me, receiptRequester, receiptsDB, headersDB,
		indexDB, incorporatedResults, receipts, approvals, seals, assigner, validator,
		requiredApprovalsForSealConstruction, emergencySealingActive, receiptsChannel, approvalsChannel, approvalResponsesChannel, approvalConduit)
	if err != nil {
		return nil, fmt.Errorf("failed to init matching engine: %w", err)
	}

	return c, nil
}

// Process sends event into channel with pending events. Generally speaking shouldn't lock for too long.
func (c *Core2) Process(originID flow.Identifier, event interface{}) error {
	c.pendingEventSink <- &Event{
		OriginID: originID,
		Msg:      event,
	}
	return nil
}

// processEvents is processor of pending events which drives events from networking layer to business logic in `Engine`.
// Effectively consumes messages from networking layer and dispatches them into corresponding sinks which are connected with `Engine`.
// Should be run as a separate goroutine.
func (c *Core2) processEvents() {
	fetchEvent := func(queue *deque.Deque, sink EventSink) (*Event, EventSink) {
		event, ok := queue.PopFront()
		if !ok {
			return nil, nil
		}
		return event.(*Event), sink
	}

	for {
		pendingReceipt, receiptSink := fetchEvent(&c.pendingReceipts, c.receiptSink)
		pendingApproval, approvalSink := fetchEvent(&c.pendingApprovals, c.approvalSink)
		pendingApprovalResponse, approvalResponseSink := fetchEvent(&c.pendingApprovalResponses, c.approvalResponseSink)
		select {
		case event := <-c.pendingEventSink:
			c.processPendingEvent(event)
		case receiptSink <- pendingReceipt:
			continue
		case approvalSink <- pendingApproval:
			continue
		case approvalResponseSink <- pendingApprovalResponse:
			continue
		case <-c.unit.Quit():
			return
		}
	}
}

// processPendingEvent saves pending event in corresponding queue for further processing by `Engine`.
// While this function runs in separate goroutine it shouldn't do heavy processing to maintain efficient data polling/pushing.
func (c *Core2) processPendingEvent(event *Event) {
	switch event.Msg.(type) {
	case *flow.ExecutionReceipt:
		c.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		if c.pendingReceipts.Len() < defaultReceiptQueueCapacity {
			c.pendingReceipts.PushBack(event)
		}
	case *flow.ResultApproval:
		c.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if c.pendingApprovals.Len() < defaultApprovalQueueCapacity {
			c.pendingApprovals.PushBack(event)
		}
	case *messages.ApprovalResponse:
		c.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if c.pendingApprovalResponses.Len() < defaultApprovalResponseQueueCapacity {
			c.pendingApprovalResponses.PushBack(event)
		}
	}
}

// SubmitLocal submits an event originating on the local node.
func (c *Core2) SubmitLocal(event interface{}) {
	c.Submit(c.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (c *Core2) Submit(originID flow.Identifier, event interface{}) {
	err := c.Process(originID, event)
	if err != nil {
		engine.LogError(c.log, err)
	}
}

// ProcessLocal processes an event originating on the local node.
func (c *Core2) ProcessLocal(event interface{}) error {
	return c.Process(c.me.NodeID(), event)
}

// HandleReceipt pipes explicitly requested receipts to the process function.
// Receipts can come from this function or the receipt provider setup in the
// engine constructor.
func (c *Core2) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {
	c.log.Debug().Msg("received receipt from requester engine")

	err := c.Process(originID, receipt)
	if err != nil {
		c.log.Error().Err(err).Hex("origin", originID[:]).Msg("could not process receipt")
	}
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (c *Core2) Ready() <-chan struct{} {
	started := make(chan struct{})
	c.unit.Launch(func() {
		close(started)
		c.processEvents()
	})
	<-started
	return c.engine.Ready()
}

func (c *Core2) Done() <-chan struct{} {
	<-c.unit.Done()
	return c.engine.Done()
}
