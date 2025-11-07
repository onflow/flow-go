// Package ingestion2 implements a modular ingestion engine responsible for
// orchestrating the processing of finalized blockchain data and receiving
// execution receipts from the network.
//
// The Engine coordinates several internal workers, each dedicated to a specific task:
//   - Receiving and persisting execution receipts from the network.
//   - Subscribing to finalized block events.
//   - Synchronizing collections associated with finalized blocks.
package ingestion2

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/storage"
)

// defaultQueueCapacity is a capacity for the execution receipt message queue
const defaultQueueCapacity = 10_000

type Engine struct {
	*component.ComponentManager

	log zerolog.Logger

	finalizedBlockProcessor *FinalizedBlockProcessor
	collectionSyncer        *Syncer

	messageHandler           *engine.MessageHandler
	executionReceiptsQueue   *engine.FifoMessageStore
	receipts                 storage.ExecutionReceipts
	collectionExecutedMetric module.CollectionExecutedMetric
}

var _ network.MessageProcessor = (*Engine)(nil)

func New(
	log zerolog.Logger,
	net network.EngineRegistry,
	finalizedBlockProcessor *FinalizedBlockProcessor,
	collectionSyncer *Syncer,
	receipts storage.ExecutionReceipts,
	collectionExecutedMetric module.CollectionExecutedMetric,
) (*Engine, error) {
	executionReceiptsRawQueue, err := fifoqueue.NewFifoQueue(defaultQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not create execution receipts queue: %w", err)
	}
	executionReceiptsQueue := &engine.FifoMessageStore{FifoQueue: executionReceiptsRawQueue}
	messageHandler := engine.NewMessageHandler(
		log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*flow.ExecutionReceipt)
				return ok
			},
			Store: executionReceiptsQueue,
		},
	)

	e := &Engine{
		log:                      log.With().Str("engine", "ingestion2").Logger(),
		finalizedBlockProcessor:  finalizedBlockProcessor,
		collectionSyncer:         collectionSyncer,
		messageHandler:           messageHandler,
		executionReceiptsQueue:   executionReceiptsQueue,
		receipts:                 receipts,
		collectionExecutedMetric: collectionExecutedMetric,
	}

	// register our workers which are basically consumers of different kinds of data.
	// engine notifies workers when new data is available so that they can start processing them.
	builder := component.NewComponentManagerBuilder().
		AddWorker(e.messageHandlerLoop).
		AddWorker(e.finalizedBlockProcessor.StartWorkerLoop)
	e.ComponentManager = builder.Build()

	// engine gets execution receipts from channels.ReceiveReceipts channel
	_, err = net.Register(channels.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine in network to receive execution receipts: %w", err)
	}

	return e, nil
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
//
// No errors are expected during normal operations.
func (e *Engine) Process(chanName channels.Channel, originID flow.Identifier, event interface{}) error {
	select {
	case <-e.ComponentManager.ShutdownSignal():
		return component.ErrComponentShutdown
	default:
	}

	//TODO: we don't need this type switch as message handler has this check under the hood
	switch event.(type) {
	case *flow.ExecutionReceipt:
		err := e.messageHandler.Process(originID, event)
		return err
	default:
		return fmt.Errorf("got invalid event type (%T) from %s channel", event, chanName)
	}
}

// messageHandlerLoop reacts to message handler notifications and processes available execution receipts
// once notification has arrived.
func (e *Engine) messageHandlerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.messageHandler.GetNotifier():
			err := e.processAvailableExecutionReceipts(ctx)
			if err != nil {
				// if an error reaches this point, it is unexpected
				ctx.Throw(err)
				return
			}
		}
	}
}

// processAvailableExecutionReceipts processes available execution receipts in the queue and handles it.
// It continues processing until all enqueued receipts are handled or the context is canceled.
//
// No errors are expected during normal operations.
func (e *Engine) processAvailableExecutionReceipts(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		msg, ok := e.executionReceiptsQueue.Get()
		if !ok {
			return nil
		}

		receipt := msg.Payload.(*flow.ExecutionReceipt)
		if err := e.persistExecutionReceipt(receipt); err != nil {
			return err
		}
	}
}

// persistExecutionReceipt persists the execution receipt.
//
// No errors are expected during normal operations.
func (e *Engine) persistExecutionReceipt(receipt *flow.ExecutionReceipt) error {
	// persist the execution receipt locally, storing will also index the receipt
	err := e.receipts.Store(receipt)
	if err != nil {
		return fmt.Errorf("failed to store execution receipt: %w", err)
	}

	e.collectionExecutedMetric.ExecutionReceiptReceived(receipt)
	return nil
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated.
// Receives block finalized events from the finalization distributor and forwards them to the consumer.
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	e.finalizedBlockProcessor.Notify()
	e.collectionSyncer.OnFinalizedBlock()
}
