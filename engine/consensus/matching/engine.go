package matching

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

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

// Engine is a wrapper for matching `Core` which implements logic for
// queuing and filtering network messages which later will be processed by matching engine.
// Purpose of this struct is to provide an efficient way how to consume messages from network layer and pass
// them to `Core`. Engine runs 2 separate gorourtines that perform pre-processing and consuming messages by Core.
type Engine struct {
	unit                                 *engine.Unit
	log                                  zerolog.Logger
	me                                   module.Local
	core                                 *Core
	cacheMetrics                         module.MempoolMetrics
	engineMetrics                        module.EngineMetrics
	pendingReceipts                      *fifoqueue.FifoQueue
	pendingApprovals                     *fifoqueue.FifoQueue
	pendingRequestedApprovals            *fifoqueue.FifoQueue
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
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
	}

	// FIFO queue for inbound receipts
	var err error
	e.pendingReceipts, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultReceiptQueueCapacity),
		fifoqueue.WithLengthObserver(func(len uint32) { mempool.MempoolEntries(metrics.ResourceReceiptQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound receipts: %w", err)
	}

	// FIFO queue for broadcasted approvals
	e.pendingApprovals, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultApprovalQueueCapacity),
		fifoqueue.WithLengthObserver(func(len uint32) { mempool.MempoolEntries(metrics.ResourceApprovalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound approvals: %w", err)
	}

	// FiFo queue for requested approvals
	e.pendingRequestedApprovals, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultApprovalResponseQueueCapacity),
		fifoqueue.WithLengthObserver(func(len uint32) { mempool.MempoolEntries(metrics.ResourceApprovalResponseQueue, uint(len)) }),
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

	e.core, err = NewCore(log, engineMetrics, tracer, mempool, conMetrics, state, me, receiptRequester, receiptsDB, headersDB,
		indexDB, incorporatedResults, receipts, approvals, seals, pendingReceipts, assigner, receiptValidator, approvalValidator,
		requiredApprovalsForSealConstruction, emergencySealingActive, approvalConduit)
	if err != nil {
		return nil, fmt.Errorf("failed to init matching engine: %w", err)
	}

	return e, nil
}

// Process sends event into channel with pending events. Generally speaking shouldn't lock for too long.
func (e *Engine) Process(originID flow.Identifier, message interface{}) error {
	select {
	case <-e.unit.Quit():
		return nil
	default: // fallthrough
	}

	switch event := message.(type) {
	case *flow.ExecutionReceipt:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		e.pendingReceipts.TailChannel() <- Event{originID, event}
	case *flow.ResultApproval:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			return nil // drop approvals, if we don't require them to construct seals
		}
		e.pendingApprovals.TailChannel() <- Event{originID, event}
	case *messages.ApprovalResponse:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			return nil // drop approvals, if we don't require them to construct seals
		}
		e.pendingRequestedApprovals.TailChannel() <- Event{originID, event.Approval}
	}
	return nil
}

// consumeQueuedEvents consumes events from the inbound queues
func (e *Engine) consumeQueuedEvents() {
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
	}, 2*time.Second, 10*time.Second)

	for {
		select {
		case <-e.unit.Quit():
			return
		default: // fallthrough
		}

		var err error
		select {
		case event := <-e.pendingReceipts.HeadChannel():
			receiptEvent := event.(*Event)
			err = e.core.OnReceipt(receiptEvent.OriginID, receiptEvent.Msg.(*flow.ExecutionReceipt))
			e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		case event := <-e.pendingApprovals.HeadChannel():
			approvalEvent := event.(*Event)
			err = e.core.OnApproval(approvalEvent.OriginID, approvalEvent.Msg.(*flow.ResultApproval))
			e.engineMetrics.MessageHandled(metrics.EngineMatching, metrics.MessageResultApproval)
		case event := <-e.pendingApprovals.HeadChannel():
			approvalEvent := event.(*Event)
			err = e.core.OnApproval(approvalEvent.OriginID, approvalEvent.Msg.(*flow.ResultApproval))
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
	wg.Add(1)
	e.unit.Launch(func() {
		wg.Done()
		e.consumeQueuedEvents()
	})
	return e.unit.Ready(func() {
		wg.Wait()
	})
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}
