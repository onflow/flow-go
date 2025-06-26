package optimistic_sync

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	//
	// Expected Errors:
	//   - context.Canceled: when the context is canceled
	//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
	//
	// CAUTION: not concurrency safe! Run must only be called once.
	Run(context.Context) error

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
	log             zerolog.Logger
	executionResult *flow.ExecutionResult
	stateReceiver   PipelineStateReceiver
	parent          PipelineStateProvider
	stateNotifier   engine.Notifier
	core            Core
	state           State

	// externally managed state
	// stored using atomics to avoid blocking the result forest during updates
	isSealed    *atomic.Bool
	isAbandoned *atomic.Bool

	// cancelFn is used to cancel the pipeline's context to abort processing.
	// it is initialized during Run, but can be called at any time via the abandon method which is
	// called by the results forest
	// stored using atomics to avoid blocking the result forest during updates
	cancelFn *atomic.Pointer[context.CancelFunc]

	// mu protects the state variable
	mu sync.RWMutex
}

// NewPipeline creates a new processing pipeline.
// The pipeline is initialized in the Pending state.
func NewPipeline(
	log zerolog.Logger,
	executionResult *flow.ExecutionResult,
	core Core,
	isSealed bool,
	parent PipelineStateProvider,
	stateReceiver PipelineStateReceiver,
) *PipelineImpl {
	log = log.With().
		Str("component", "pipeline").
		Str("execution_result_id", executionResult.ExecutionDataID.String()).
		Str("block_id", executionResult.BlockID.String()).
		Logger()

	return &PipelineImpl{
		log:             log,
		executionResult: executionResult,
		stateReceiver:   stateReceiver,
		core:            core,
		state:           StatePending,
		parent:          parent,
		isSealed:        atomic.NewBool(isSealed),
		isAbandoned:     atomic.NewBool(false),
		cancelFn:        atomic.NewPointer[context.CancelFunc](nil),
		stateNotifier:   engine.NewNotifier(),
	}
}

// Run starts the pipeline processing and blocks until completion or context cancellation.
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
//
// CAUTION: not concurrency safe! Run must only be called once.
func (p *PipelineImpl) Run(ctx context.Context) error {
	pipelineCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	p.cancelFn.Store(&cancel)

	if p.isAbandoned.Load() {
		cancel()
		if err := p.transitionTo(StateAbandoned); err != nil {
			return fmt.Errorf("failed to transition to abandoned state during initialization: %w", err)
		}
	} else {
		if err := p.transitionTo(StateReady); err != nil {
			return fmt.Errorf("failed to transition to ready state during initialization: %w", err)
		}
	}

	notifierChan := p.stateNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-notifierChan:
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			state := p.GetState()
			err := p.processCurrentState(pipelineCtx, state)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					return fmt.Errorf("error running pipeline: %w", err)
				}

				// the main context was canceled. shutdown without transitioning to avoid cascading
				// abandoned state updates since all pipelines may share the same root context
				if ctx.Err() != nil {
					return fmt.Errorf("running pipeline failed with context canceled: %w", err)
				}

				// the pipeline's context was canceled. transition to abandoned and process the state
				// update before returning
				if p.GetState() != StateAbandoned {
					if err := p.transitionTo(StateAbandoned); err != nil {
						return fmt.Errorf("failed to transition to abandoned state during context cancellation: %w", err)
					}
				}
				continue
			}

			if state.IsTerminal() {
				return nil
			}
		}
	}
}

// GetState returns the current state of the pipeline.
func (p *PipelineImpl) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// SetSealed marks the data as sealed, which enables transitioning from StateWaitingPersist to StatePersisting.
func (p *PipelineImpl) SetSealed() {
	// Note: do not add locking here to avoid blocking the results forest.
	if p.isSealed.CompareAndSwap(false, true) {
		p.stateNotifier.Notify()
	}
}

// OnParentStateUpdated updates the pipeline's state based on the provided parent state.
func (p *PipelineImpl) OnParentStateUpdated(parentState State) {
	// Note: do not add locking here to avoid blocking the results forest.
	if parentState == StateAbandoned {
		p.abandon()
	}

	p.stateNotifier.Notify()
}

// Abandon marks the pipeline as abandoned
func (p *PipelineImpl) Abandon() {
	// Note: do not add locking here to avoid blocking the results forest.
	p.abandon()
	p.stateNotifier.Notify()
}

// abandon marks the pipeline as abandoned and cancels its context, which abort processing.
// if the pipeline is already abandoned, this is a no-op.
func (p *PipelineImpl) abandon() {
	p.isAbandoned.Store(true)
	p.cancel()
}

// cancel cancels the pipeline's context if it has been initialized, which abort processing.
func (p *PipelineImpl) cancel() {
	cancelFn := p.cancelFn.Load()
	if cancelFn != nil {
		cancel := *cancelFn
		cancel()
	}
}

// processCurrentState handles the current state and transitions to the next state if possible.
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) processCurrentState(ctx context.Context, currentState State) (err error) {
	start := time.Now()
	defer func() {
		p.log.Debug().Err(err).
			Str("state", currentState.String()).
			Dur("duration", time.Since(start)).
			Msg("completed processing step")
	}()

	switch currentState {
	case StateReady:
		return p.processReady()
	case StateDownloading:
		return p.processDownloading(ctx)
	case StateIndexing:
		return p.processIndexing()
	case StateWaitingPersist:
		return p.processWaitingPersist()
	case StatePersisting:
		return p.processPersisting()
	case StateAbandoned:
		return p.processAbandoned()
	case StateComplete:
		return nil // nothing to do
	default:
		return fmt.Errorf("invalid pipeline state: %s", currentState)
	}
}

// processReady handles the Ready state and transitions to StateDownloading if possible.
//
// No errors are expected during normal operations
func (p *PipelineImpl) processReady() error {
	switch p.parent.GetState() {
	case StateDownloading, StateIndexing, StateWaitingPersist, StatePersisting, StateComplete:
		return p.transitionTo(StateDownloading)
	case StatePending, StateReady:
		// this pipeline should not be started before the parent, but it's possible there is a race
		// starting the pipelines in the worker pool. pause and wait for the parent to start.
		return nil
	default:
		// its unexpected for the parent to be in any other state. this most likely indicates there's a bug
		return p.transitionTo(StateAbandoned)
	}
}

// processDownloading handles the Downloading state and transitions to StateIndexing if successful.
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) processDownloading(ctx context.Context) error {
	if p.checkAbandoned() {
		return p.transitionTo(StateAbandoned)
	}

	if err := p.core.Download(ctx); err != nil {
		return err
	}

	return p.transitionTo(StateIndexing)
}

// processIndexing handles the Indexing state and transitions to StateWaitingPersist if successful.
//
// No errors are expected during normal operations
func (p *PipelineImpl) processIndexing() error {
	if p.checkAbandoned() {
		return p.transitionTo(StateAbandoned)
	}

	if err := p.core.Index(); err != nil {
		return err
	}

	return p.transitionTo(StateWaitingPersist)
}

// processWaitingPersist handles the WaitingPersist state and transitions to StatePersisting if possible.
// Conditions for transition:
//  1. The result must be sealed
//  2. The parent pipeline must be complete
//
// No errors are expected during normal operations
func (p *PipelineImpl) processWaitingPersist() error {
	if p.checkAbandoned() {
		return p.transitionTo(StateAbandoned)
	}

	if p.isSealed.Load() && p.parent.GetState() == StateComplete {
		return p.transitionTo(StatePersisting)
	}
	return nil
}

// processPersisting handles the Persisting state and transitions to StateComplete if successful.
//
// No errors are expected during normal operations
func (p *PipelineImpl) processPersisting() error {
	if err := p.core.Persist(); err != nil {
		return err
	}

	return p.transitionTo(StateComplete)
}

// processAbandoned handles the Abandoned state
//
// No errors are expected during normal operations
func (p *PipelineImpl) processAbandoned() error {
	if err := p.core.Abandon(); err != nil {
		return err
	}

	return nil
}

// transitionTo transitions the pipeline to the given state and broadcasts
// the state change to children pipelines.
func (p *PipelineImpl) transitionTo(newState State) error {
	hasChange, err := p.setState(newState)
	if err != nil {
		return err
	}

	if hasChange {
		// send notification for all states except ready.
		// Ready is not needed since it's the initial state and does not impact children's state machines.
		if newState != StateReady {
			p.stateReceiver.OnStateUpdated(newState)
		}
		p.stateNotifier.Notify()
	}

	return nil
}

// setState sets the state of the pipeline and logs the transition.
func (p *PipelineImpl) setState(newState State) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// transitioning to the same state is a no-op
	if p.state == newState {
		return false, nil
	}

	if err := p.validateTransition(p.state, newState); err != nil {
		return false, fmt.Errorf("failed to transition from %s to %s: %w", p.state, newState, err)
	}

	oldState := p.state
	p.state = newState

	p.log.Debug().
		Str("old_state", oldState.String()).
		Str("new_state", newState.String()).
		Msg("pipeline state transition")

	return true, nil
}

// checkAbandoned returns true if the pipeline or its parent are abandoned.
func (p *PipelineImpl) checkAbandoned() bool {
	if p.isAbandoned.Load() {
		return true
	}

	if p.parent.GetState() == StateAbandoned {
		return true
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.state == StateAbandoned
}

// validateTransition validates the transition from the current state to the new state.
//
// Expected Errors:
//   - ErrInvalidTransition: when the transition is invalid
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) validateTransition(currentState State, newState State) error {
	switch newState {
	case StateReady:
		if currentState == StatePending {
			return nil
		}

	case StateDownloading:
		if currentState == StateReady {
			return nil
		}

	case StateIndexing:
		if currentState == StateDownloading {
			return nil
		}

	case StateWaitingPersist:
		if currentState == StateIndexing {
			return nil
		}

	case StatePersisting:
		if currentState == StateWaitingPersist {
			return nil
		}

	case StateComplete:
		if currentState == StatePersisting {
			return nil
		}

	case StateAbandoned:
		// Note: it does not make sense to transition to abandoned from persisting or completed since to be in either state:
		// 1. the parent must be completed
		// 2. the pipeline's result must be sealed
		// At that point, there are no conditions that would cause the pipeline be abandoned
		switch currentState {
		case StatePending, StateReady, StateDownloading, StateIndexing, StateWaitingPersist:
			return nil
		}

	default:
		return fmt.Errorf("invalid transition to state: %s", newState)
	}

	return ErrInvalidTransition
}
