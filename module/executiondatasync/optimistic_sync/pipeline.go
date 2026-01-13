package optimistic_sync

import (
	"context"
	"errors"

	"github.com/onflow/flow-go/model/flow"
)

// ErrInvalidTransition is returned when a state transition is invalid.
var ErrInvalidTransition = errors.New("invalid state transition")

// PipelineFactory is a factory object for creating new Pipeline instances.
type PipelineFactory interface {
	// NewPipeline creates a new pipeline for a given execution result.
	NewPipeline(result *flow.ExecutionResult) Pipeline

	// NewCompletedPipeline creates a new pipeline for a given execution result that has already
	// completed processing. This should only be used for the latest persisted sealed result during
	// bootstrapping.
	NewCompletedPipeline(result *flow.ExecutionResult) Pipeline
}

// PipelineStateProvider is an interface that provides a pipeline's state.
type PipelineStateProvider interface {
	// GetState returns the current state of the pipeline.
	GetState() State
}

// PipelineStateConsumer is a receiver of the pipeline state updates.
// PipelineStateConsumer implementations must be
// - NON-BLOCKING and consume the state updates without noteworthy delay
type PipelineStateConsumer interface {
	// OnStateUpdated is called when a pipeline's state changes to notify the receiver of the new state.
	// This method is will be called in the same goroutine that runs the pipeline, so it must not block.
	OnStateUpdated(newState State)
}

// Pipeline represents a pipelined state machine for a processing single ExecutionResult.
// The state machine is initialized in the Pending state.
//
// The state machine is designed to be run in a single goroutine. The Run method must only be called once.
type Pipeline interface {
	PipelineStateProvider

	// Run starts the pipeline processing and blocks until completion or context cancellation.
	// NOT CONCURRENCY SAFE! Run must only be called once.
	//
	// Expected error returns during normal operations:
	//   - [context.Canceled]: when the context is canceled
	Run(ctx context.Context, core Core, parentState State) error

	// SetSealed marks the pipeline's result as sealed, which enables transitioning from StateWaitingPersist to StatePersisting.
	SetSealed()

	// OnParentStateUpdated updates the pipeline's parent's state.
	OnParentStateUpdated(parentState State)

	// Abandon marks the pipeline as abandoned.
	Abandon()
}
