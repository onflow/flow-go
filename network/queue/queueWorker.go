package queue

import (
	"context"

	"github.com/onflow/flow-go/network"
)

const DefaultNumWorkers = 5

// worker de-queues an item from the queue if available and calls the callback function endlessly
// if no item is available, it blocks
func worker(ctx context.Context, queue network.MessageQueue, callback func(interface{})) {
	for {
		// blocking call
		item := queue.Remove()
		select {
		case <-ctx.Done():
			return
		default:
			callback(item)
		}
	}
}

// CreateQueueWorkers creates queue workers to read from the queue
func CreateQueueWorkers(ctx context.Context, numWorks uint64, queue network.MessageQueue, callback func(interface{})) {
	for i := uint64(0); i < numWorks; i++ {
		go worker(ctx, queue, callback)
	}
}
