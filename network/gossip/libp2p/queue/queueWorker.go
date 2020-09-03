package queue

import "context"

const DefaultNumWorkers = 5

// worker de-queues an item from the queue if available and calls the callback function endlessly
// if no item is available, it blocks
func worker(ctx context.Context, queue MessageQueue, callback func(interface{})) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// blocking call
			item := queue.Remove()
			callback(item)

		}
	}
}

// CreateQueueWorkers creates queue workers to read from the queue
func CreateQueueWorkers(ctx context.Context, numWorks uint64, queue MessageQueue, callback func(interface{})) {
	for i := uint64(0); i < numWorks; i++ {
		go worker(ctx, queue, callback)
	}
}
