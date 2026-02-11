package optimistic_sync

import (
	"context"
	"errors"
)

// ErrInvalidTransition is returned when a state transition is invalid.
var ErrInvalidTransition = errors.New("invalid state transition")

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

// Pipeline represents a processing pipelined state machine for a single ExecutionResult.
// The state machine is initialized in the Pending state.
//
// The state machine is designed to be run in a single goroutine. The Run method must only be called once.
type Pipeline interface {
	PipelineStateProvider

	// Run starts the pipeline processing and blocks until completion or context cancellation.
	// CAUTION: not concurrency safe! Run must only be called once.
	//
	// Expected Errors:
	//   - context.Canceled: when the context is canceled
	//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
	Run(ctx context.Context, core Core, parentState State) error

	// SetSealed marks the pipeline's result as sealed, which enables transitioning from StateWaitingPersist to StatePersisting.
	SetSealed()

	// OnParentStateUpdated updates the pipeline's parent's state.
	OnParentStateUpdated(parentState State)

	// Abandon marks the pipeline as abandoned.
	Abandon()
}
