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
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// defaultReceiptQueueCapacity maximum capacity of receipts queue
const defaultReceiptQueueCapacity = 10000

// Engine is a wrapper struct for `Core` which implements consensus algorithm.
// Engine is responsible for handling incoming messages, queueing for processing, broadcasting proposals.
type Engine struct {
	unit                       *engine.Unit
	log                        zerolog.Logger
	me                         module.Local
	core                       sealing.MatchingCore
	state                      protocol.State
	results                    storage.ExecutionResults
	payloads                   storage.Payloads
	metrics                    module.EngineMetrics
	inboundReceiptNotifier     engine.Notifier
	pendingReceipts            *fifoqueue.FifoQueue
	finalizationEventsNotifier engine.Notifier
}

func NewEngine(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	engineMetrics module.EngineMetrics,
	mempool module.MempoolMetrics,
	state protocol.State,
	payloads storage.Payloads,
	results storage.ExecutionResults,
	core sealing.MatchingCore) (*Engine, error) {

	// FIFO queue for execution receipts
	receiptsQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultReceiptQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { mempool.MempoolEntries(metrics.ResourceBlockProposalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound receipts: %w", err)
	}

	e := &Engine{
		log:                        log.With().Str("engine", "matching.Engine").Logger(),
		unit:                       engine.NewUnit(),
		me:                         me,
		core:                       core,
		state:                      state,
		payloads:                   payloads,
		results:                    results,
		metrics:                    engineMetrics,
		inboundReceiptNotifier:     engine.NewNotifier(),
		finalizationEventsNotifier: engine.NewNotifier(),
		pendingReceipts:            receiptsQueue,
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
	e.unit.Launch(e.finalizationProcessingLoop)
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
	receipt, ok := event.(*flow.ExecutionReceipt)
	if !ok {
		return fmt.Errorf("input message of incompatible type: %T, origin: %x", event, originID[:])
	}
	e.metrics.MessageReceived(metrics.EngineSealing, metrics.MessageExecutionReceipt)
	e.pendingReceipts.Push(receipt)
	e.inboundReceiptNotifier.Notify()
	return nil
}

// HandleReceipt ingests receipts from the Requester module.
func (e *Engine) HandleReceipt(originID flow.Identifier, receipt flow.Entity) {
	e.log.Debug().Msg("received receipt from requester engine")
	e.metrics.MessageReceived(metrics.EngineSealing, metrics.MessageExecutionReceipt)
	e.pendingReceipts.Push(receipt)
	e.inboundReceiptNotifier.Notify()
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(finalizedBlockID flow.Identifier) {
	e.finalizationEventsNotifier.Notify()
	e.processFinalizedReceipts(finalizedBlockID)
}

// processFinalizedReceipts selects receipts that were included into finalized block and submits them
// for further processing by matching core.
// Without the logic below, the sealing engine would produce IncorporatedResults
// only from receipts received directly from ENs. sealing Core would not know about
// Receipts that are incorporated by other nodes in their blocks blocks (but never
// received directly from the EN).
func (e *Engine) processFinalizedReceipts(finalizedBlockID flow.Identifier) {
	e.unit.Launch(func() {
		payload, err := e.payloads.ByBlockID(finalizedBlockID)
		if err != nil {
			e.log.Fatal().Err(err).Msgf("could not retrieve payload for block %v", finalizedBlockID)
		}
		resultsById := payload.Results.Lookup()
		for _, meta := range payload.Receipts {
			// Generally speaking we are interested in receipts that were included in block together with execution results
			// but since we require two receipts from different ENs before sealing we need to add every receipt included in block.
			result, ok := resultsById[meta.ResultID]
			if !ok {
				result, err = e.results.ByID(meta.ResultID)
				// error at this point means that we have corrupted state or serious bug which allows including
				// invalid receipts into finalized blocks
				if err != nil {
					e.log.Fatal().Err(err).Msgf("could not retrieve result %v", meta.ResultID)
				}
			}

			receipt := flow.ExecutionReceiptFromMeta(*meta, *result)
			added := e.pendingReceipts.Push(receipt)
			if !added {
				// Not being able to queue an execution receipt is a fatal edge case. It might happen, if the
				// queue capacity is depleted. However, we cannot dropped the execution receipt, because there
				// is no way that an execution receipt can be re-added later once dropped.
				e.log.Fatal().Msg("failed to queue execution receipt")
			}
		}
		e.inboundReceiptNotifier.Notify()
	})
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop() {
	finalizationNotifier := e.finalizationEventsNotifier.Channel()
	for {
		select {
		case <-e.unit.Quit():
			return
		case <-finalizationNotifier:
			err := e.processLatestFinalizedEvent()
			if err != nil {
				e.log.Fatal().Err(err).Msg("could not process latest finalized event")
			}
		}
	}
}

func (e *Engine) loop() {
	c := e.inboundReceiptNotifier.Channel()

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

// processLatestFinalizedEvent performs processing of latest finalized event propagating it to core
func (e *Engine) processLatestFinalizedEvent() error {
	err := e.core.OnBlockFinalization()
	if err != nil {
		return fmt.Errorf("could not process last finalized event: %w", err)
	}
	return nil
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

		msg, ok := e.pendingReceipts.Pop()
		if ok {
			err := e.core.ProcessReceipt(msg.(*flow.ExecutionReceipt))
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
