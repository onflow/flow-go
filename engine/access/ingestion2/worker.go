package ingestion2

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/queue"
)

// PipelineWorkerPool processes execution result containers through their associated pipelines.
// It continuously pulls work from a priority message queue and executes the pipeline
// for each container, ensuring ancestor pipelines are executed first.
type PipelineWorkerPool struct {
	resultsForest *ResultsForest
	messageQueue  *queue.PriorityMessageQueue[*ExecutionResultContainer]
}

// NewPipelineWorkerPool creates a new instance of PipelineWorkerPool.
//
// Parameters:
//   - resultsForest: the forest containing execution result containers
//   - messageQueue: the priority queue containing containers to be processed
//
// Returns:
//   - *PipelineWorkerPool: the newly created worker pool instance
func NewPipelineWorkerPool(resultsForest *ResultsForest, messageQueue *queue.PriorityMessageQueue[*ExecutionResultContainer]) *PipelineWorkerPool {
	return &PipelineWorkerPool{
		resultsForest: resultsForest,
		messageQueue:  messageQueue,
	}
}

// WorkerLoop is a component.ComponentWorker that continuously processes execution result containers.
// This method should be added to a component.ComponentManager for each worker in the pool.
// Each worker pulls containers from the message queue and executes their associated pipelines.
//
// Parameters:
//   - ctx: the irrecoverable context that controls the worker lifecycle
//   - ready: function to signal that the worker is ready to process work
func (e *PipelineWorkerPool) WorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	ch := e.messageQueue.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		container, ok := e.getWork(ctx, ch)
		if !ok {
			continue
		}

		if err := e.executePipeline(ctx, container); err != nil {
			ctx.Throw(err)
		}
	}
}

// getWork retrieves the next execution result container from the message queue.
// If no container is immediately available, it blocks until one becomes available
// or the context is cancelled.
//
// Parameters:
//   - ctx: the context that controls the blocking behavior
//   - ch: the channel that signals when new work is available
//
// Returns:
//   - *ExecutionResultContainer: the next container to process, or nil if context was cancelled
//   - bool: true if a container was retrieved, false if the context was cancelled
func (e *PipelineWorkerPool) getWork(ctx context.Context, ch <-chan struct{}) (*ExecutionResultContainer, bool) {
	for {
		// if there is a message available, return it immediately
		container, ok := e.messageQueue.Pop()
		if ok {
			return container, true
		}

		// otherwise, wait for a signal or a context cancellation
		select {
		case <-ctx.Done():
			return nil, false
		case <-ch:
		}
	}
}

// executePipeline executes the pipeline for the given execution result container.
// It ensures the parent container exists in the forest and runs the pipeline with
// the parent's state as input.
//
// Parameters:
//   - ctx: the context for the pipeline execution
//   - container: the execution result container to process
//
// Returns:
//   - error: any error that occurred during pipeline execution
//
// No errors are expected during normal operation.
func (e *PipelineWorkerPool) executePipeline(ctx context.Context, container *ExecutionResultContainer) error {
	if container.Pipeline().GetState() == optimistic_sync.StateAbandoned {
		return nil // don't execute abandoned state machines
	}

	parentID, _ := container.Parent()
	parent, ok := e.resultsForest.GetContainer(parentID)
	if !ok {
		// parent MUST exist in the forest, otherwise this container should not be in the queue.
		// this is either a bug or the forest has corrupted state.
		return fmt.Errorf("parent container not found for (result: %s, parent: %s)", container.resultID, parentID)
	}

	// TODO: decide on a method of communicating initial parent state. alternative is notifications + pull
	core := optimistic_sync.NewCore()
	parentState := parent.Pipeline().GetState()
	if err := container.Pipeline().Run(ctx, core, parentState); err != nil {
		return fmt.Errorf("failed to run pipeline: %w", err)
	}

	return nil
}
