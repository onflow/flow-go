package optimistic_sync

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
)

// Pipeline represents a processing pipelined state machine for a single ExecutionResult.
//
// The state machine is initialized in the Pending state, and can transition to Abandoned at any time
// if the parent pipeline is abandoned.
//
// The state machine is designed to be run in a single goroutine. The Run method must only be called once.
type Pipeline interface {
	Run(context.Context, Core) error
	GetState() State
	SetSealed()
	OnParentStateUpdated(State)
}

// StateUpdatePublisher is a function that publishes state updates
type StateUpdatePublisher func(state State)

var _ Pipeline = (*PipelineImpl)(nil)

// PipelineImpl implements the Pipeline interface
type PipelineImpl struct {
	log             zerolog.Logger
	executionResult *flow.ExecutionResult
	statePublisher  StateUpdatePublisher
	core            Core

	state       State
	parentState State
	isSealed    bool

	stateNotifier engine.Notifier
	cancel        context.CancelFunc

	mu sync.RWMutex
}

// NewPipeline creates a new processing pipeline.
// The pipeline is initialized in the Pending state.
func NewPipeline(
	log zerolog.Logger,
	isSealed bool,
	executionResult *flow.ExecutionResult,
	core Core,
	statePublisher StateUpdatePublisher,
) *PipelineImpl {
	log = log.With().
		Str("component", "pipeline").
		Str("execution_result_id", executionResult.ExecutionDataID.String()).
		Str("block_id", executionResult.BlockID.String()).
		Logger()

	return &PipelineImpl{
		log:             log,
		statePublisher:  statePublisher,
		state:           StatePending,
		isSealed:        isSealed,
		stateNotifier:   engine.NewNotifier(),
		core:            core,
		executionResult: executionResult,
	}
}

// Run starts the pipeline processing and blocks until completion or context cancellation.
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
//
// CAUTION: not concurrency safe! Run must only be called once.
func (p *PipelineImpl) Run(parentCtx context.Context, core Core) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	p.mu.Lock()
	p.core = core
	p.state = StateReady
	p.cancel = cancel
	p.mu.Unlock()

	notifierChan := p.stateNotifier.Channel()

	// Trigger initial check
	p.stateNotifier.Notify()

	for {
		select {
		case <-parentCtx.Done():
			return parentCtx.Err()

		case <-notifierChan:
			processing, err := p.processCurrentState(ctx)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					return err
				}

				// the parent context was canceled. shutdown without transitioning to avoid cascading
				// abandoned state updates since all pipelines may share the same root context
				if parentCtx.Err() != nil {
					return err
				}

				// the pipeline's context was canceled. transition to abandoned and process the state
				// update before returning
				if p.GetState() != StateAbandoned {
					p.transitionTo(StateAbandoned)
				}
				continue
			}

			if !processing {
				// terminal state reached
				return ctx.Err()
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
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isSealed {
		p.isSealed = true
		p.stateNotifier.Notify()
	}
}

// OnParentStateUpdated updates the pipeline's state based on the provided parent state.
//
// Side effects:
//   - If the parent pipeline is abandoned and the current pipeline is not already in the abandoned state,
//     1. this pipeline's context will be canceled.
//     2. the state update will eventually be broadcast to children pipelines.
func (p *PipelineImpl) OnParentStateUpdated(parentState State) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.parentState = parentState

	// If parent is abandoned, abandon this pipeline
	if parentState == StateAbandoned {
		if p.cancel != nil {
			p.cancel()
		}
	}

	p.stateNotifier.Notify()
}

// processCurrentState handles the current state and transitions to the next state if possible.
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) processCurrentState(ctx context.Context) (bool, error) {
	currentState := p.GetState()

	switch currentState {
	case StateReady:
		return p.processReady(), nil
	case StateDownloading:
		return p.processDownloading(ctx)
	case StateIndexing:
		return p.processIndexing()
	case StateWaitingPersist:
		return p.processWaitingPersist(), nil
	case StatePersisting:
		return p.processPersisting()
	case StateAbandoned:
		return p.processAbandoned()
	case StateComplete:
		// Terminal state
		return false, nil
	default:
		return false, fmt.Errorf("invalid pipeline state: %s", currentState)
	}
}

// transitionTo transitions the pipeline to the given state and broadcasts
// the state change to children pipelines.
func (p *PipelineImpl) transitionTo(newState State) {
	p.setState(newState)
	p.statePublisher(newState)

	if newState == StateComplete {
		return
	}

	p.stateNotifier.Notify()
}

// setState sets the state of the pipeline and logs the transition.
func (p *PipelineImpl) setState(newState State) {
	p.mu.Lock()
	defer p.mu.Unlock()

	oldState := p.state
	p.state = newState

	p.log.Debug().
		Str("old_state", oldState.String()).
		Str("new_state", newState.String()).
		Msg("pipeline state transition")
}

// processReady handles the Ready state and transitions to StateDownloading if possible.
func (p *PipelineImpl) processReady() bool {
	if p.canStartDownloading() {
		p.transitionTo(StateDownloading)
		return true
	}
	return true
}

// processDownloading handles the Downloading state.
// It executes the download function and transitions to StateIndexing if successful.
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (p *PipelineImpl) processDownloading(ctx context.Context) (bool, error) {
	p.log.Debug().Msg("starting download step")

	if err := p.core.Download(ctx); err != nil {
		p.log.Error().Err(err).Msg("download step failed")
		return false, err
	}

	p.log.Debug().Msg("download step completed")

	if !p.canStartIndexing() {
		// If we can't transition to indexing after successful download, abandon
		p.transitionTo(StateAbandoned)
		return true, nil
	}
	p.transitionTo(StateIndexing)
	return true, nil
}

// processIndexing handles the Indexing state.
// It executes the index function and transitions to StateWaitingPersist if possible.
//
// No errors are expected during normal operations
func (p *PipelineImpl) processIndexing() (bool, error) {
	p.log.Debug().Msg("starting index step")

	if err := p.core.Index(); err != nil {
		p.log.Error().Err(err).Msg("index step failed")
		return false, err
	}

	p.log.Debug().Msg("index step completed")

	if !p.canWaitForPersist() {
		// If we can't transition to waiting for persist after successful indexing, abandon
		p.transitionTo(StateAbandoned)
		return true, nil
	}

	p.transitionTo(StateWaitingPersist)
	return true, nil
}

// processWaitingPersist handles the WaitingPersist state.
// It checks if the conditions for persisting are met and transitions to StatePersisting if possible.
func (p *PipelineImpl) processWaitingPersist() bool {
	transitionReady, abandoned := p.canStartPersisting()
	if abandoned {
		p.transitionTo(StateAbandoned)
		return true
	}
	if transitionReady {
		p.transitionTo(StatePersisting)
		return true
	}
	return true
}

// processPersisting handles the Persisting state.
// It executes the persist function and transitions to StateComplete if successful.
//
// No errors are expected during normal operations
func (p *PipelineImpl) processPersisting() (bool, error) {
	p.log.Debug().Msg("starting persist step")

	if err := p.core.Persist(); err != nil {
		p.log.Error().Err(err).Msg("persist step failed")
		return false, err
	}

	p.log.Debug().Msg("persist step completed")
	p.transitionTo(StateComplete)
	return false, nil
}

// processAbandoned handles the Abandoned state.
// It cancels the pipeline context and calls core.Abandon.
//
// No errors are expected during normal operations
func (p *PipelineImpl) processAbandoned() (bool, error) {
	p.log.Debug().Msg("processing abandoned state")

	if err := p.core.Abandon(); err != nil {
		p.log.Error().Err(err).Msg("abandon step failed")
		return false, err
	}

	p.log.Debug().Msg("abandon step completed")
	return false, nil
}

// canStartDownloading checks if the pipeline can transition from Ready to Downloading.
//
// Conditions for transition:
//  1. The current state must be Ready
//  2. The parent pipeline must be in an active state (StateDownloading, StateIndexing,
//     StateWaitingPersist, StatePersisting, or StateComplete)
func (p *PipelineImpl) canStartDownloading() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.state != StateReady {
		return false
	}

	switch p.parentState {
	case StateDownloading, StateIndexing, StateWaitingPersist, StatePersisting, StateComplete:
		return true
	default:
		return false
	}
}

// canStartIndexing checks if the pipeline can transition from Downloading to Indexing.
//
// Conditions for transition:
// 1. The current state must be Downloading
// 2. The parent pipeline must not be abandoned
func (p *PipelineImpl) canStartIndexing() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.state == StateDownloading && p.parentState != StateAbandoned
}

// canWaitForPersist checks if the pipeline can transition from Indexing to WaitingPersist.
//
// Conditions for transition:
// 1. The current state must be Indexing
// 2. The parent pipeline must not be abandoned
func (p *PipelineImpl) canWaitForPersist() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.state == StateIndexing && p.parentState != StateAbandoned
}

// canStartPersisting checks if the pipeline can transition from WaitingPersist to Persisting.
//
// Conditions for transition:
// 1. The current state must be WaitingPersist
// 2. The result must be sealed
// 3. The parent pipeline must be complete
func (p *PipelineImpl) canStartPersisting() (transitionReady bool, abandoned bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	transitionReady = p.state == StateWaitingPersist && p.isSealed && p.parentState == StateComplete
	abandoned = p.state == StateAbandoned || p.parentState == StateAbandoned

	return
}
