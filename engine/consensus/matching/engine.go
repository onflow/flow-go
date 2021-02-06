package matching

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
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

// Engine is a wrapper for matching `Core` which implements logic for
// queuing and filtering network messages which later will be processed by matching engine.
// Purpose of this struct is to provide an efficient way how to consume messages from network layer and pass
// them to `Core`. Engine runs 2 separate gorourtines that perform pre-processing and consuming messages by Core.
type Engine struct {
	notifications.NoopConsumer

	unit                                 *engine.Unit
	log                                  zerolog.Logger
	me                                   module.Local
	core                                 *Core
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
	receiptsDB                           storage.ExecutionReceipts
	payloadsDB                           storage.Payloads
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
	payloadsDB storage.Payloads,
	incorporatedResults mempool.IncorporatedResults,
	receipts mempool.ExecutionTree,
	approvals mempool.Approvals,
	seals mempool.IncorporatedResultSeals,
	pendingReceipts mempool.PendingReceipts,
	assigner module.ChunkAssigner,
	receiptValidator module.ReceiptValidator,
	approvalValidator module.ApprovalValidator,
	requiredApprovalsForSealConstruction uint,
	emergencySealingActive bool) (*Engine, error) {
	e := &Engine{
		unit:                                 engine.NewUnit(),
		log:                                  log,
		me:                                   me,
		core:                                 nil,
		engineMetrics:                        engineMetrics,
		cacheMetrics:                         mempool,
		receiptSink:                          make(EventSink),
		approvalSink:                         make(EventSink),
		requestedApprovalSink:                make(EventSink),
		pendingEventSink:                     make(EventSink),
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		receiptsDB:                           receiptsDB,
		payloadsDB:                           payloadsDB,
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

	// register engine to the channel for requesting missing approvals
	approvalConduit, err := net.Register(engine.RequestApprovalsByChunk, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for requesting approvals: %w", err)
	}

	e.core, err = NewCore(log, engineMetrics, tracer, mempool, conMetrics, net, state, me, receiptRequester, receiptsDB, headersDB,
		indexDB, incorporatedResults, receipts, approvals, seals, pendingReceipts, assigner, receiptValidator, approvalValidator,
		requiredApprovalsForSealConstruction, emergencySealingActive, approvalConduit)
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
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		e.pendingReceipts.Push(event)
	case *flow.ResultApproval:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return
		}
		e.pendingApprovals.Push(event)
	case *messages.ApprovalResponse:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			// if we don't require approvals to construct a seal, don't even process approvals.
			return
		}
		e.pendingRequestedApprovals.Push(event)
	}
}

// consumeEvents consumes events that are ready to be processed.
func (e *Engine) consumeEvents() {
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
		var err error
		select {
		case event := <-e.receiptSink:
			err = e.core.OnReceipt(event.OriginID, event.Msg.(*flow.ExecutionReceipt))
			e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		case event := <-e.approvalSink:
			err = e.core.OnApproval(event.OriginID, event.Msg.(*flow.ResultApproval))
			e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageResultApproval)
		case event := <-e.requestedApprovalSink:
			err = e.core.OnApproval(event.OriginID, &event.Msg.(*messages.ApprovalResponse).Approval)
			e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageResultApproval)
		case <-checkSealingTicker:
			err = e.core.CheckSealing()
		case <-e.unit.Quit():
			return
		}
		if err != nil {
			// Public methods of `Core` are supposed to handle all errors internally.
			// Here if error happens it means that internal state is corrupted or we have caught
			// exception while processing. In such case best just to abort the node.
			e.log.Fatal().Err(err).Msgf("fatal internal error in matching core logic")
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

// OnFinalizedBlock implements the callback from the protocol state to notify a block
// is finalized, it guarantees every finalized block will be called at least once.
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	// we index the execution receipts by the executed block ID only for all finalized blocks
	// that guarantees if we could retrieve the receipt by the index, then the receipts
	// must be for a finalized blocks.
	err := e.indexReceipts(block.BlockID)
	if err != nil {
		e.log.Fatal().Err(err).Hex("block_id", block.BlockID[:]).
			Msg("could not index receipts for block")
	}
}

func (e *Engine) indexReceipts(blockID flow.Identifier) error {
	payload, err := e.payloadsDB.ByBlockID(blockID)

	if err != nil {
		return fmt.Errorf("could not get block payload: %w", err)
	}

	for _, receipt := range payload.Receipts {
		err := e.receiptsDB.IndexByExecutor(receipt)
		if err != nil {
			return fmt.Errorf("could not index receipt by executor, receipt id: %v: %w", receipt.ID(), err)
		}
	}

	return nil
}
