package matching

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	sealing "github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
)

// defaultReceiptQueueCapacity maximum capacity of receipts queue
const defaultReceiptQueueCapacity = 10000

// defaultFinalizationQueueCapacity maximum capacity of finalization queue
const defaultFinalizationQueueCapacity = 100

// Engine is a wrapper struct for `Core` which implements consensus algorithm.
// Engine is responsible for handling incoming messages, queueing for processing, broadcasting proposals.
type Engine struct {
	unit                      *engine.Unit
	log                       zerolog.Logger
	me                        module.Local
	core                      sealing.MatchingCore
	pendingReceipts           *engine.FifoMessageStore
	pendingFinalizationEvents *engine.FifoMessageStore
	messageHandler            *engine.MessageHandler
}

func NewEngine(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	engineMetrics module.EngineMetrics,
	mempool module.MempoolMetrics,
	core sealing.MatchingCore) (*Engine, error) {

	// FIFO queue for execution receipts
	receiptsQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultReceiptQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { mempool.MempoolEntries(metrics.ResourceBlockProposalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound receipts: %w", err)
	}

	pendingReceipts := &engine.FifoMessageStore{
		FifoQueue: receiptsQueue,
	}

	// FIFO queue for finalization events
	finalizationQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultFinalizationQueueCapacity),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound finalization events: %w", err)
	}

	pendingFinalizationEvents := &engine.FifoMessageStore{
		FifoQueue: finalizationQueue,
	}

	// define message queueing behaviour
	handler := engine.NewMessageHandler(
		log.With().Str("matching", "engine").Logger(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*flow.ExecutionReceipt)
				if ok {
					engineMetrics.MessageReceived(metrics.EngineSealing, metrics.MessageExecutionReceipt)
				}
				return ok
			},
			Store: pendingReceipts,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(flow.Identifier)
				return ok
			},
			Store: pendingFinalizationEvents,
		},
	)

	e := &Engine{
		log:                       log.With().Str("matching", "engine").Logger(),
		unit:                      engine.NewUnit(),
		me:                        me,
		core:                      core,
		messageHandler:            handler,
		pendingReceipts:           pendingReceipts,
		pendingFinalizationEvents: pendingFinalizationEvents,
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
	e.unit.Launch(e.loop)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
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
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.messageHandler.Process(originID, event)
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

func (e *Engine) HandleFinalizedBlock(finalizedBlockID flow.Identifier) {
	err := e.messageHandler.Process(e.me.NodeID(), finalizedBlockID)
	if err != nil {
		e.log.Error().Err(err).Msg("could not process finalized block")
	}
}

func (e *Engine) loop() {
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-e.messageHandler.GetNotifier():
			err := e.processAvailableMessages()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing queued message")
			}
		}
	}
}

func (e *Engine) processAvailableMessages() error {

	for {
		msg, ok := e.pendingFinalizationEvents.Get()
		if ok {
			err := e.core.ProcessFinalizedBlock(msg.Payload.(flow.Identifier))
			if err != nil {
				return fmt.Errorf("could not process finalized block: %w", err)
			}
			continue
		}

		msg, ok = e.pendingReceipts.Get()
		if ok {
			err := e.core.ProcessReceipt(msg.OriginID, msg.Payload.(*flow.ExecutionReceipt))
			if err != nil {
				return fmt.Errorf("could not handle execution receipt: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}
