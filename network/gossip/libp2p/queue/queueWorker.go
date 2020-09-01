package queue

import "context"

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

func CreateQueueWorkers(ctx context.Context, numWorks uint64, queue MessageQueue, callback func(interface{})) {
	for i := uint64(0); i < numWorks; i++ {
		go worker(ctx, queue, callback)
	}
}
