package optimistic_sync

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
)

var (
	// ErrInvalidTransition is returned when a state transition is invalid.
	ErrInvalidTransition = errors.New("invalid state transition")
)

// PipelineStateProvider is an interface that provides a pipeline's state.
type PipelineStateProvider interface {
	// GetState returns the current state of the pipeline.
	GetState() State
}

// PipelineStateReceiver is a receiver of the pipeline state updates.
type PipelineStateReceiver interface {
	// OnStateUpdated is called when a pipeline's state changes.
	OnStateUpdated(State)
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
	Run(context.Context, Core, PipelineStateProvider) error

	// SetSealed marks the pipeline's result as sealed, which enables transitioning from StateWaitingPersist to StatePersisting.
	SetSealed()

	// OnParentStateUpdated updates the pipeline's parent's state.
	OnParentStateUpdated(State)

	// Abandon marks the pipeline as abandoned.
	Abandon()
}

var _ Pipeline = (*PipelineImpl)(nil)

// PipelineImpl implements the Pipeline interface
type PipelineImpl struct {
	log                  zerolog.Logger
	executionResult      *flow.ExecutionResult
	stateReceiver        PipelineStateReceiver
	stateChangedNotifier engine.Notifier
	core                 Core
	parent               PipelineStateProvider

	// The following fields are accessed externally. they are stored using atomics to avoid
	// blocking the caller.

	state       *atomic.Int32
	isSealed    *atomic.Bool
	isAbandoned *atomic.Bool
	isIndexed   *atomic.Bool
}

// NewPipeline creates a new processing pipeline.
// The pipeline is initialized in the Pending state.
func NewPipeline(
	log zerolog.Logger,
	executionResult *flow.ExecutionResult,
	isSealed bool,
	stateReceiver PipelineStateReceiver,
) *PipelineImpl {
	log = log.With().
		Str("component", "pipeline").
		Str("execution_result_id", executionResult.ExecutionDataID.String()).
		Str("block_id", executionResult.BlockID.String()).
		Logger()

	return &PipelineImpl{
		log:                  log,
		executionResult:      executionResult,
		stateReceiver:        stateReceiver,
		state:                atomic.NewInt32(int32(StatePending)),
		isSealed:             atomic.NewBool(isSealed),
		stateChangedNotifier: engine.NewNotifier(),
	}
}

// Run starts the pipeline processing and blocks until completion or context cancellation.
// CAUTION: not concurrency safe! Run must only be called once.
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) Run(ctx context.Context, core Core, parent PipelineStateProvider) error {
	pipelineCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	p.core = core
	p.parent = parent

	// try to start processing in case we are able to.
	if err := p.onStartProcessing(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stateChangedNotifier.Channel():
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// if parent got abandoned no point to continue and we just go to the abandoned state and perform cleanup logic.
			if p.checkAbandoned() {
				if err := p.transitionTo(StateAbandoned); err != nil {
					return fmt.Errorf("failed to transition to abandoned state during initialization: %w", err)
				}
				continue
			}

			currentState := p.GetState()
			switch currentState {
			case StatePending:
				if err := p.onStartProcessing(pipelineCtx); err != nil {
					return err
				}
			case StateProcessing:
				if err := p.onProcessing(); err != nil {
					return err
				}
			case StateWaitingPersist:
				if err := p.onPersistChanges(); err != nil {
					return err
				}
			case StateAbandoned:
				return p.core.Abandon()
			case StateComplete:
				return nil // terminate
			default:
				return fmt.Errorf("invalid pipeline state: %s", currentState)
			}
		}
	}
}

func (p *PipelineImpl) onStartProcessing(ctx context.Context) error {
	switch p.parent.GetState() {
	case StateProcessing, StateWaitingPersist, StateComplete:
		err := p.transitionTo(StateProcessing)
		if err != nil {
			return err
		}
		childCtx, cancel := context.WithCancelCause(ctx)
		go func() {
			err := p.performDownload(childCtx)
			if err != nil {
				cancel(err)
			}
		}()
	case StatePending:
		return nil
	case StateAbandoned:
		return p.transitionTo(StateAbandoned)
	default:
		// its unexpected for the parent to be in any other state. this most likely indicates there's a bug
		return fmt.Errorf("unexpected parent state: %s", p.parent.GetState())
	}
	return nil
}

func (p *PipelineImpl) onProcessing() error {
	if p.isIndexed.Load() {
		return p.transitionTo(StateWaitingPersist)
	}
	return nil
}

func (p *PipelineImpl) onPersistChanges() error {
	if p.isSealed.Load() && p.parent.GetState() == StateComplete {
		if err := p.core.Persist(); err != nil {
			return err
		}
		return p.transitionTo(StateComplete)
	} else {
		return nil
	}
}

// checkAbandoned returns true if the pipeline or its parent are abandoned.
func (p *PipelineImpl) checkAbandoned() bool {
	if p.isAbandoned.Load() {
		return true
	}
	if p.parent.GetState() == StateAbandoned {
		return true
	}
	return p.GetState() == StateAbandoned
}

// GetState returns the current state of the pipeline.
func (p *PipelineImpl) GetState() State {
	return State(p.state.Load())
}

// SetSealed marks the data as sealed, which enables transitioning from StateWaitingPersist to StatePersisting.
func (p *PipelineImpl) SetSealed() {
	// Note: do not use a mutex here to avoid blocking the results forest.
	if p.isSealed.CompareAndSwap(false, true) {
		p.stateChangedNotifier.Notify()
	}
}

// OnParentStateUpdated updates the pipeline's state based on the provided parent state.
func (p *PipelineImpl) OnParentStateUpdated(parentState State) {
	p.stateChangedNotifier.Notify()
}

// Abandon marks the pipeline as abandoned
// This will cause the pipeline to eventually transition to the Abandoned state and halt processing
func (p *PipelineImpl) Abandon() {
	if p.isAbandoned.CompareAndSwap(false, true) {
		p.stateChangedNotifier.Notify()
	}
}

// performDownload handles the Downloading state and transitions to StateIndexing if successful.
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) performDownload(ctx context.Context) error {
	if err := p.core.Download(ctx); err != nil {
		return err
	}
	if err := p.core.Index(); err != nil {
		return err
	}
	if p.isIndexed.CompareAndSwap(false, true) {
		p.stateChangedNotifier.Notify()
	}
	return nil
}

// transitionTo transitions the pipeline to the given state and broadcasts
// the state change to children pipelines.
//
// Expected Errors:
//   - ErrInvalidTransition: when the transition is invalid
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) transitionTo(newState State) error {
	hasChange, err := p.setState(newState)
	if err != nil {
		return err
	}

	if hasChange {
		// send notification for all states except ready.
		// Ready is not needed since it's the initial state and does not impact children's state machines.
		p.stateReceiver.OnStateUpdated(newState)
		p.stateChangedNotifier.Notify()
	}

	return nil
}

// setState sets the state of the pipeline and logs the transition.
// Returns true if the state was changed, false otherwise.
//
// Expected Errors:
//   - ErrInvalidTransition: when the state transition is invalid
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) setState(newState State) (bool, error) {
	currentState := p.GetState()

	// transitioning to the same state is a no-op
	if currentState == newState {
		return false, nil
	}

	if err := p.validateTransition(currentState, newState); err != nil {
		return false, fmt.Errorf("failed to transition from %s to %s: %w", currentState, newState, err)
	}

	if !p.state.CompareAndSwap(int32(currentState), int32(newState)) {
		// Note: this should never happen since state is only updated within the Run goroutine.
		return false, fmt.Errorf("failed to transition from %s to %s", currentState, newState)
	}

	p.log.Debug().
		Str("old_state", currentState.String()).
		Str("new_state", newState.String()).
		Msg("pipeline state transition")

	return true, nil
}

// validateTransition validates the transition from the current state to the new state.
//
// Expected Errors:
//   - ErrInvalidTransition: when the transition is invalid
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) validateTransition(currentState State, newState State) error {
	switch newState {
	case StateProcessing:
		if currentState == StatePending {
			return nil
		}
	case StateWaitingPersist:
		if currentState == StateProcessing {
			return nil
		}
	case StateComplete:
		if currentState == StateWaitingPersist {
			return nil
		}
	case StateAbandoned:
		// Note: it does not make sense to transition to abandoned from persisting or completed since to be in either state:
		// 1. the parent must be completed
		// 2. the pipeline's result must be sealed
		// At that point, there are no conditions that would cause the pipeline be abandoned
		switch currentState {
		case StatePending, StateWaitingPersist:
			return nil
		}

	default:
		return fmt.Errorf("invalid transition to state: %s", newState)
	}

	return ErrInvalidTransition
}
