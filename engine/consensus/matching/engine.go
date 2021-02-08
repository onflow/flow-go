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
)

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
	requiredApprovalsForSealConstruction uint

	// channels for inbound messages
	trapdoor           Trapdoor // trapdoor is used for suspending worker thread
	pendingReceipts    *FifoQueue
	pendingApprovals   *FifoQueue
	requestedApprovals *FifoQueue
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
	e := &Engine{
		unit:                                 engine.NewUnit(),
		log:                                  log,
		me:                                   me,
		core:                                 nil,
		engineMetrics:                        engineMetrics,
		cacheMetrics:                         mempool,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		trapdoor:                             NewTrapdoor(false),
	}

	// FiFo queue for inbound receipts
	var err error
	e.pendingReceipts, err = NewFifoQueue(
		WithCapacity(defaultReceiptQueueCapacity),
		WithLenMetric(func(len int) { mempool.MempoolEntries(metrics.ResourceReceiptQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound receipts: %w", err)
	}

	// FiFo queue for broadcasted approvals
	e.pendingApprovals, err = NewFifoQueue(
		WithCapacity(defaultApprovalQueueCapacity),
		WithLenMetric(func(len int) { mempool.MempoolEntries(metrics.ResourceApprovalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound approvals: %w", err)
	}

	// FiFo queue for requested approvals
	e.requestedApprovals, err = NewFifoQueue(
		WithCapacity(defaultApprovalResponseQueueCapacity),
		WithLenMetric(func(len int) { mempool.MempoolEntries(metrics.ResourceApprovalResponseQueue, uint(len)) }),
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
		indexDB, incorporatedResults, receipts, approvals, seals, assigner, validator,
		requiredApprovalsForSealConstruction, emergencySealingActive, approvalConduit)
	if err != nil {
		return nil, fmt.Errorf("failed to init matching engine: %w", err)
	}

	return e, nil
}

// Process sends event into channel with pending events. Generally speaking shouldn't lock for too long.
func (e *Engine) Process(originID flow.Identifier, message interface{}) error {
	switch message.(type) {
	case *flow.ExecutionReceipt:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		e.pendingReceipts.Push(originID, message)
	case *flow.ResultApproval:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			return nil // drop approvals, if we don't require them to construct seals
		}
		e.pendingApprovals.Push(originID, message)
	case *messages.ApprovalResponse:
		e.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		if e.requiredApprovalsForSealConstruction < 1 {
			return nil // drop approvals, if we don't require them to construct seals
		}
		e.requestedApprovals.Push(originID, message)
	}
	e.trapdoor.Activate()
	return nil
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
	e.unit.LaunchPeriodically(
		func() {
			checkSealingTicker <- struct{}{}
			e.trapdoor.Activate()
		},
		2*time.Second, 120*time.Second,
	)

	for {
		checkForInputs := true
		for checkForInputs {
			// check if engine was terminated first
			select {
			case <-e.unit.Quit():
				return
			default: // fall through
			}

			select {
			case <-checkSealingTicker:
				e.core.checkSealing()
			default: // fall through to process events
			}

			checkForInputs = e.processNextEvent()
		}

		e.trapdoor.Pass()
	}
}

func (e *Engine) processNextEvent() bool {
	originID, message, ok := e.pendingReceipts.Pop()
	if ok {
		err := e.core.onReceipt(originID, message.(*flow.ExecutionReceipt))
		if err != nil {
			// TODO: we probably want to check the type of the error here
			// * If the message was invalid (e.g. NewInvalidInputError), which is expected as part
			//   of normal operations in an untrusted environment, we can just log an error.
			// * However, if we receive an error indicating internal node failure or state
			//   internal state corruption, we should probably crash the node.
			e.log.Error().Err(err).Hex("origin", originID[:]).Msgf("could not process receipt")
		}
		return true
	}

	originID, message, ok = e.requestedApprovals.Pop()
	if ok {
		reqApproval := message.(*messages.ApprovalResponse)
		err := e.core.onApproval(originID, &reqApproval.Approval)
		if err != nil {
			// TODO: we probably want to check the type of the error here
			// * If the message was invalid (e.g. NewInvalidInputError), which is expected as part
			//   of normal operations in an untrusted environment, we can just log an error.
			// * However, if we receive an error indicating internal node failure or state
			//   internal state corruption, we should probably crash the node.
			e.log.Error().Err(err).Hex("origin", originID[:]).Msgf("could not process approval")
		}
		return true
	}

	originID, message, ok = e.pendingApprovals.Pop()
	if ok {
		err := e.core.onApproval(originID, message.(*flow.ResultApproval))
		if err != nil {
			// TODO: we probably want to check the type of the error here
			// * If the message was invalid (e.g. NewInvalidInputError), which is expected as part
			//   of normal operations in an untrusted environment, we can just log an error.
			// * However, if we receive an error indicating internal node failure or state
			//   internal state corruption, we should probably crash the node.
			e.log.Error().Err(err).Hex("origin", originID[:]).Msgf("could not process approval")
		}
		return true
	}

	return false
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
		e.consumeEvents()
	})
	return e.unit.Ready(func() {
		wg.Wait()
	})
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}
