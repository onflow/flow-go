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
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
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
	unit                       *engine.Unit
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
	net network.Network,
	me module.Local,
	engineMetrics module.EngineMetrics,
	mempool module.MempoolMetrics,
	state protocol.State,
	receipts storage.ExecutionReceipts,
	index storage.Index,
	core sealing.MatchingCore) (*Engine, error) {

	// FIFO queue for execution receipts
	receiptsQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultReceiptQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { mempool.MempoolEntries(metrics.ResourceBlockProposalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound receipts: %w", err)
	}

	pendingIncorporatedBlocks, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultIncorporatedBlockQueueCapacity))
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for incorporated block events: %w", err)
	}

	e := &Engine{
		log:                        log.With().Str("engine", "matching.Engine").Logger(),
		unit:                       engine.NewUnit(),
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
	_, err = net.Register(engine.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.inboundEventsProcessingLoop)
	e.unit.Launch(e.finalizationProcessingLoop)
	e.unit.Launch(e.blockIncorporatedEventsProcessingLoop)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	err := e.ProcessLocal(event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	err := e.Process(channel, originID, event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	err := e.process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
			return nil
		}
		return fmt.Errorf("unexpected error while processing engine message: %w", err)
	}
	return nil
}

// process events for the matching engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	receipt, ok := event.(*flow.ExecutionReceipt)
	if !ok {
		return fmt.Errorf("no matching processor for message of type %T from origin %x: %w", event, originID[:],
			engine.IncompatibleInputTypeError)
	}
	e.metrics.MessageReceived(metrics.EngineSealing, metrics.MessageExecutionReceipt)
	e.pendingReceipts.Push(receipt)
	e.inboundEventsNotifier.Notify()
	return nil
}

// HandleReceipt ingests receipts from the Requester module.
func (e *Engine) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {
	e.log.Debug().Msg("received receipt from requester engine")
	err := e.process(originID, receipt)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event from requester module")
	}
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
// Receipts that are incorporated by other nodes in their blocks blocks (but never
// received directly from the EN).
func (e *Engine) processIncorporatedBlock(finalizedBlockID flow.Identifier) error {
	index, err := e.index.ByBlockID(finalizedBlockID)
	if err != nil {
		e.log.Fatal().Err(err).Msgf("could not retrieve payload index for block %v", finalizedBlockID)
	}
	for _, receiptID := range index.ReceiptIDs {
		receipt, err := e.receipts.ByID(receiptID)
		if err != nil {
			return fmt.Errorf("could not retrieve receipt incorporated in block %v: %w", finalizedBlockID, err)
		}
		e.pendingReceipts.Push(receipt)
	}
	e.inboundEventsNotifier.Notify()
	return nil
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop() {
	finalizationNotifier := e.finalizationEventsNotifier.Channel()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-finalizationNotifier:
			err := e.core.OnBlockFinalization()
			if err != nil {
				e.log.Fatal().Err(err).Msg("could not process last finalized event")
			}
		}
	}
}

// blockIncorporatedEventsProcessingLoop is a separate goroutine for processing block incorporated events
func (e *Engine) blockIncorporatedEventsProcessingLoop() {
	c := e.blockIncorporatedNotifier.Channel()

	for {
		select {
		case <-e.unit.Quit():
			return
		case <-c:
			err := e.processBlockIncorporatedEvents()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing block incorporated queued message")
			}
		}
	}
}

func (e *Engine) inboundEventsProcessingLoop() {
	c := e.inboundEventsNotifier.Channel()

	for {
		select {
		case <-e.unit.Quit():
			return
		case <-c:
			err := e.processAvailableEvents()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing queued message")
			}
		}
	}
}

// processBlockIncorporatedEvents performs processing of block incorporated hot stuff events
func (e *Engine) processBlockIncorporatedEvents() error {
	for {
		select {
		case <-e.unit.Quit():
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

// processAvailableEvents processes _all_ available events (untrusted messages
// from other nodes as well as internally trusted
func (e *Engine) processAvailableEvents() error {
	for {
		select {
		case <-e.unit.Quit():
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

		msg, ok = e.pendingReceipts.Pop()
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
