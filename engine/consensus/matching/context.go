package matching

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/concurrentqueue"
)

type Event struct {
	OriginID flow.Identifier
	Msg      interface{}
}

// EventProvider is an interface which provides polling mechanism for
// events that are ready to be processedengine/consensus/matching/context_test.go
type EventProvider interface {
	Poll(maxBatchSize int) []*Event
}

type concurrentQueueProvider struct {
	q concurrentqueue.ConcurrentQueue
}

func (c *concurrentQueueProvider) Push(event *Event) {
	c.q.Push(event)
}

func (c *concurrentQueueProvider) Poll(maxBatchSize int) []*Event {
	r, notEmpty := c.q.PopBatch(maxBatchSize)
	if !notEmpty {
		return nil
	}
	transformed := make([]*Event, len(r))
	for i, item := range r {
		transformed[i] = item.(*Event)
	}
	return transformed
}

// EngineContext is a wrapper for matching engine which implements logic for
// queuing network messages which later will be processed by matching engine.
type EngineContext struct {
	log                      zerolog.Logger
	me                       module.Local
	engine                   *Engine
	receiptsProvider         *concurrentQueueProvider
	approvalsProvider        *concurrentQueueProvider
	approvalResponseProvider *concurrentQueueProvider
}

func NewEngineContext(log zerolog.Logger,
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
	emergencySealingActive bool) (*EngineContext, error) {
	c := &EngineContext{
		log:                      log,
		me:                       me,
		engine:                   nil,
		receiptsProvider:         &concurrentQueueProvider{},
		approvalsProvider:        &concurrentQueueProvider{},
		approvalResponseProvider: &concurrentQueueProvider{},
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

	c.engine, err = New(log, engineMetrics, tracer, mempool, conMetrics, state, me, receiptRequester, receiptsDB, headersDB,
		indexDB, incorporatedResults, receipts, approvals, seals, assigner, validator,
		requiredApprovalsForSealConstruction, emergencySealingActive, c.receiptsProvider, c.approvalResponseProvider, c.approvalsProvider, approvalConduit)
	if err != nil {
		return nil, fmt.Errorf("failed to init matching engine: %w", err)
	}

	return c, nil
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (c *EngineContext) Process(originID flow.Identifier, event interface{}) error {
	switch event.(type) {
	case *flow.ExecutionReceipt:
		//c.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageExecutionReceipt)
		c.receiptsProvider.Push(&Event{
			OriginID: originID,
			Msg:      event,
		})
		return nil
	case *flow.ResultApproval:
		//c.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		c.approvalsProvider.Push(&Event{
			OriginID: originID,
			Msg:      event,
		})
		return nil
	case *messages.ApprovalResponse:
		//c.engineMetrics.MessageReceived(metrics.EngineMatching, metrics.MessageResultApproval)
		c.approvalResponseProvider.Push(&Event{
			OriginID: originID,
			Msg:      event,
		})
		return nil
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// SubmitLocal submits an event originating on the local node.
func (c *EngineContext) SubmitLocal(event interface{}) {
	c.Submit(c.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (c *EngineContext) Submit(originID flow.Identifier, event interface{}) {
	err := c.Process(originID, event)
	if err != nil {
		engine.LogError(c.log, err)
	}
}

// ProcessLocal processes an event originating on the local node.
func (c *EngineContext) ProcessLocal(event interface{}) error {
	return c.Process(c.me.NodeID(), event)
}

// HandleReceipt pipes explicitly requested receipts to the process function.
// Receipts can come from this function or the receipt provider setup in the
// engine constructor.
func (c *EngineContext) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {
	c.log.Debug().Msg("received receipt from requester engine")

	err := c.Process(originID, receipt)
	if err != nil {
		c.log.Error().Err(err).Hex("origin", originID[:]).Msg("could not process receipt")
	}
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (c *EngineContext) Ready() <-chan struct{} {
	return c.engine.Ready()
}

func (c *EngineContext) Done() <-chan struct{} {
	return c.engine.Done()
}
