package ingestion2

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

const defaultQueueCapacity = 10_000

type ExecutionReceiptConsumer struct {
	log                      zerolog.Logger
	collectionExecutedMetric module.CollectionExecutedMetric

	messageHandler       *engine.MessageHandler
	receiptsMessageQueue engine.MessageStore
	receipts             storage.ExecutionReceipts
}

func NewExecutionReceiptConsumer(
	log zerolog.Logger,
	collectionExecutedMetric module.CollectionExecutedMetric,
	receipts storage.ExecutionReceipts,
) (*ExecutionReceiptConsumer, error) {
	receiptsRawQueue, err := fifoqueue.NewFifoQueue(defaultQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not create execution receipts queue: %w", err)
	}

	receiptsMessageQueue := &engine.FifoMessageStore{FifoQueue: receiptsRawQueue}
	messageHandler := engine.NewMessageHandler(
		log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*flow.ExecutionReceipt)
				return ok
			},
			Store: receiptsMessageQueue,
		},
	)

	return &ExecutionReceiptConsumer{
		log:                      log,
		collectionExecutedMetric: collectionExecutedMetric,
		receipts:                 receipts,
		messageHandler:           messageHandler,
	}, nil
}

// Notify processes a new execution receipt payload and notifies the consumer to begin processing queued messages.
//
// No errors are expected during normal operations.
func (c *ExecutionReceiptConsumer) Notify(originID flow.Identifier, payload interface{}) error {
	// call to Process also notifies underlying notifier to which we subscribed in the worker loop
	err := c.messageHandler.Process(originID, payload)
	return err
}

// StartWorkerLoop starts the execution receipt processing loop. It waits for notifications
// and processes available receipts until the context is cancelled or an irrecoverable error occurs.
func (c *ExecutionReceiptConsumer) StartWorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.messageHandler.GetNotifier():
			err := c.processAvailableExecutionReceipts(ctx)
			if err != nil {
				// if an error reaches this point, it is unexpected
				ctx.Throw(err)
				return
			}
		}
	}
}

// processAvailableExecutionReceipts processes available execution receipts in the queue and handles it.
// It continues processing until the context is canceled.
//
// No errors are expected during normal operations.
func (c *ExecutionReceiptConsumer) processAvailableExecutionReceipts(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		msg, ok := c.receiptsMessageQueue.Get()
		if !ok {
			return nil
		}

		receipt := msg.Payload.(*flow.ExecutionReceipt)
		if err := c.persistExecutionReceipt(receipt); err != nil {
			return err
		}
	}
}

// persistExecutionReceipt persists the execution receipt.
//
// No errors are expected during normal operations.
func (c *ExecutionReceiptConsumer) persistExecutionReceipt(receipt *flow.ExecutionReceipt) error {
	// persist the execution receipt locally, storing will also index the receipt
	err := c.receipts.Store(receipt)
	if err != nil {
		return fmt.Errorf("failed to store execution receipt: %w", err)
	}

	c.collectionExecutedMetric.ExecutionReceiptReceived(receipt)
	return nil
}
