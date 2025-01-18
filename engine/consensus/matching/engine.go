package matching

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	sealing "github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// defaultReceiptQueueCapacity maximum capacity of receipts queue
const defaultReceiptQueueCapacity = 10000

// defaultIncorporatedBlockQueueCapacity maximum capacity of block incorporated events queue
const defaultIncorporatedBlockQueueCapacity = 10

// Engine is a wrapper struct for `Core` which implements consensus algorithm.
// Engine is responsible for handling incoming messages, queueing for processing, broadcasting proposals.
type Engine struct {
	component.Component
	cm                         *component.ComponentManager
	log                        zerolog.Logger
	me                         module.Local
	core                       sealing.MatchingCore
	state                      protocol.State
	receipts                   storage.ExecutionReceipts
	index                      storage.Index
	metrics                    module.EngineMetrics
	inboundEventsNotifier      engine.Notifier
	finalizationEventsNotifier engine.Notifier
	blockIncorporatedNotifier  engine.Notifier
	pendingReceipts            *fifoqueue.FifoQueue
	pendingIncorporatedBlocks  *fifoqueue.FifoQueue
}

func NewEngine(
	log zerolog.Logger,
	net network.EngineRegistry,
	me module.Local,
	engineMetrics module.EngineMetrics,
	mempool module.MempoolMetrics,
	state protocol.State,
	receipts storage.ExecutionReceipts,
	index storage.Index,
	core sealing.MatchingCore) (*Engine, error) {

	// FIFO queue for execution receipts
	receiptsQueue, err := fifoqueue.NewFifoQueue(
		defaultReceiptQueueCapacity,
		fifoqueue.WithLengthObserver(func(len int) { mempool.MempoolEntries(metrics.ResourceBlockProposalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound receipts: %w", err)
	}

	pendingIncorporatedBlocks, err := fifoqueue.NewFifoQueue(defaultIncorporatedBlockQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for incorporated block events: %w", err)
	}

	e := &Engine{
		log:                        log.With().Str("engine", "matching.Engine").Logger(),
		me:                         me,
		core:                       core,
		state:                      state,
		receipts:                   receipts,
		index:                      index,
		metrics:                    engineMetrics,
		inboundEventsNotifier:      engine.NewNotifier(),
		finalizationEventsNotifier: engine.NewNotifier(),
		blockIncorporatedNotifier:  engine.NewNotifier(),
		pendingReceipts:            receiptsQueue,
		pendingIncorporatedBlocks:  pendingIncorporatedBlocks,
	}

	// register engine with the receipt provider
	_, err = net.Register(channels.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	e.cm = component.NewComponentManagerBuilder().
		AddWorker(e.inboundEventsProcessingLoop).
		AddWorker(e.finalizationProcessingLoop).
		AddWorker(e.blockIncorporatedEventsProcessingLoop).
		Build()
	e.Component = e.cm

	return e, nil
}

// Process receives events from the network and checks their type,
// before enqueuing them to be processed by a worker in a non-blocking manner.
// No errors expected during normal operation (errors are logged instead).
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	receipt, ok := event.(*flow.ExecutionReceipt)
	if !ok {
		e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
		return nil
	}
	e.addReceiptToQueue(receipt)
	return nil
}

// addReceiptToQueue adds an execution receipt to the queue of the matching engine, to be processed by a worker
func (e *Engine) addReceiptToQueue(receipt *flow.ExecutionReceipt) {
	e.metrics.MessageReceived(metrics.EngineSealing, metrics.MessageExecutionReceipt)
	e.pendingReceipts.Push(receipt)
	e.inboundEventsNotifier.Notify()
}

// HandleReceipt ingests receipts from the Requester module, adding them to the queue.
func (e *Engine) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {
	e.log.Debug().Msg("received receipt from requester engine")
	r, ok := receipt.(*flow.ExecutionReceipt)
	if !ok {
		e.log.Fatal().Err(engine.IncompatibleInputTypeError).Msg("internal error processing event from requester module")
	}
	e.addReceiptToQueue(r)
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(*model.Block) {
	e.finalizationEventsNotifier.Notify()
}

// OnBlockIncorporated implements the `OnBlockIncorporated` callback from the `hotstuff.FinalizationConsumer`
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnBlockIncorporated(incorporatedBlock *model.Block) {
	e.pendingIncorporatedBlocks.Push(incorporatedBlock.BlockID)
	e.blockIncorporatedNotifier.Notify()
}

// processIncorporatedBlock selects receipts that were included into incorporated block and submits them
// for further processing by matching core.
// Without the logic below, the sealing engine would produce IncorporatedResults
// only from receipts received directly from ENs. sealing Core would not know about
// Receipts that are incorporated by other nodes in their blocks (but never
// received directly from the EN).
// No errors expected during normal operations.
func (e *Engine) processIncorporatedBlock(blockID flow.Identifier) error {
	index, err := e.index.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve payload index for incorporated block %v", blockID)
	}
	for _, receiptID := range index.ReceiptIDs {
		receipt, err := e.receipts.ByID(receiptID)
		if err != nil {
			return fmt.Errorf("could not retrieve receipt incorporated in block %v: %w", blockID, err)
		}
		e.pendingReceipts.Push(receipt)
	}
	e.inboundEventsNotifier.Notify()
	return nil
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	finalizationNotifier := e.finalizationEventsNotifier.Channel()
	ready()
	for {
		select {
		case <-ctx.Done():
			return
		case <-finalizationNotifier:
			err := e.core.OnBlockFinalization()
			if err != nil {
				ctx.Throw(fmt.Errorf("could not process last finalized event: %w", err))
			}
		}
	}
}

// blockIncorporatedEventsProcessingLoop is a separate goroutine for processing block incorporated events.
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

// inboundEventsProcessingLoop is a worker for processing execution receipts, received
// from the network via Process, from the Requester module via HandleReceipt, or from incorporated blocks.
func (e *Engine) inboundEventsProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	c := e.inboundEventsNotifier.Channel()
	ready()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c:
			err := e.processExecutionReceipts(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("internal error processing queued execution receipt: %w", err))
			}
		}
	}
}

// processBlockIncorporatedEvents performs processing of block incorporated hot stuff events.
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

// processExecutionReceipts processes execution receipts
// from other nodes as well as internally trusted.
// No errors expected during normal operations.
func (e *Engine) processExecutionReceipts(ctx irrecoverable.SignalerContext) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := e.pendingReceipts.Pop()
		if ok {
			err := e.core.ProcessReceipt(msg.(*flow.ExecutionReceipt))
			if err != nil {
				return fmt.Errorf("could not handle execution receipt: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the inboundEventsProcessingLoop to wait
		// for the next incoming message to arrive.
		return nil
	}
}
