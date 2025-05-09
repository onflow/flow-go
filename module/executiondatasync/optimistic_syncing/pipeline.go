package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
)

// StateUpdatePublisher is a function that publishes state updates
type StateUpdatePublisher func(resultID flow.Identifier, newState State)

// Pipeline represents a generic processing pipeline with state transitions.
// It processes data through sequential states: Ready -> Downloading -> Indexing ->
// WaitingPersist -> Persisting -> Complete, with conditions for each transition.
type Pipeline struct {
	logger         zerolog.Logger
	statePublisher StateUpdatePublisher

	mu              sync.RWMutex
	state           State
	isSealed        bool
	parentState     State
	executionResult *flow.ExecutionResult

	core          Core
	stateNotifier engine.Notifier
	cancelFn      context.CancelFunc
}

// NewPipeline creates a new processing pipeline.
// Pipelines must only be created for ExecutionResults that descend from the latest persisted sealed result.
// The pipeline is initialized in the Ready state.
//
// Parameters:
//   - logger: the logger to use for the pipeline
//   - isSealed: indicates if the pipeline's ExecutionResult is sealed
//   - executionResult: processed execution result
//   - stateUpdatePublisher: called when the pipeline needs to broadcast state updates
//
// Returns:
//   - new pipeline object
func NewPipeline(
	logger zerolog.Logger,
	isSealed bool,
	executionResult *flow.ExecutionResult,
	stateUpdatePublisher StateUpdatePublisher,
) *Pipeline {
	p := &Pipeline{
		logger: logger.With().
			Str("component", "pipeline").
			Str("execution_result_id", executionResult.ExecutionDataID.String()).
			Str("block_id", executionResult.BlockID.String()).
			Logger(),
		statePublisher:  stateUpdatePublisher,
		state:           StateUninitialized,
		isSealed:        isSealed,
		stateNotifier:   engine.NewNotifier(),
		executionResult: executionResult,
	}

	return p
}

// Run starts the pipeline processing and blocks until completion or context cancellation.
//
// This function handles the progression through the pipeline states, executing the appropriate
// processing functions at each step.
//
// When the pipeline reaches a terminal state (StateComplete or StateCanceled), the function returns.
// The function will also return if the provided context is canceled.
//
// Returns an error if any processing step fails with an irrecoverable error.
// Returns nil if processing completes successfully, reaches a terminal state,
// or if either the parent or pipeline context is canceled.
func (p *Pipeline) Run(parentCtx context.Context, core Core) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	p.mu.Lock()
	p.cancelFn = cancel
	p.state = StateReady
	p.core = core
	p.mu.Unlock()

	notifierChan := p.stateNotifier.Channel()

	// Trigger initial check
	p.stateNotifier.Notify()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-notifierChan:
			processed, err := p.processCurrentState(ctx)
			if err != nil {
				return err
			}
			if !processed {
				return nil // Terminal state reached
			}
		}
	}
}

// GetState returns the current state of the pipeline.
func (p *Pipeline) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// SetSealed marks the data as sealed, which enables transitioning from StateWaitingPersist to StatePersisting.
func (p *Pipeline) SetSealed() {
	p.mu.Lock()
	p.isSealed = true
	p.mu.Unlock()

	p.stateNotifier.Notify()
}

// OnParentStateUpdated updates the pipeline's parent state.
func (p *Pipeline) OnParentStateUpdated(parentState State) {
	p.mu.Lock()
	p.parentState = parentState
	p.mu.Unlock()

	// If our parent pipeline has aborted, we should abort too
	if parentState == StateCanceled {
		p.transitionTo(StateCanceled)
		return
	}

	p.stateNotifier.Notify()
}

// broadcastStateUpdate sends a state update via the state publisher.
// Note: this causes the state updates to propagate to all descendent pipelines synchronously.
func (p *Pipeline) broadcastStateUpdate() {
	p.statePublisher(p.executionResult.ID(), p.GetState())
}

func (p *Pipeline) cancel() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.cancelFn != nil {
		p.cancelFn()
	}
}

// processCurrentState handles the current state and transitions to the next state if possible.
// It returns false when a terminal state is reached (StateComplete or StateCanceled), true otherwise.
// Returns an error if any processing step fails.
// TODO: document expected error's once the Core logic is implemented
func (p *Pipeline) processCurrentState(ctx context.Context) (bool, error) {
	currentState := p.GetState()

	switch currentState {
	case StateReady:
		return p.processReady(), nil
	case StateDownloading:
		return p.processDownloading(ctx)
	case StateIndexing:
		return p.processIndexing(ctx)
	case StateWaitingPersist:
		return p.processWaitingPersist(), nil
	case StatePersisting:
		return p.processPersisting(ctx)
	case StateCanceled:
		return false, p.processCancel(ctx)
	case StateComplete:
		return false, nil
	default:
		// this also catches StateUninitialized since processCurrentState should never be called before
		// Run() is called.
		return false, fmt.Errorf("invalid pipeline state: %s", currentState.String())
	}
}

// transitionTo transitions the pipeline to the given state and broadcasts
// the state change to children pipelines.
func (p *Pipeline) transitionTo(newState State) {
	p.mu.Lock()
	oldState := p.state
	p.state = newState
	p.mu.Unlock()

	p.logger.Debug().
		Str("old_state", oldState.String()).
		Str("new_state", newState.String()).
		Msg("pipeline state transition")

	// Broadcast state update to children
	p.broadcastStateUpdate()

	// Trigger state check in case we can immediately transition again
	if newState != StateComplete && newState != StateCanceled {
		p.stateNotifier.Notify()
	}
}

// processReady handles the Ready state and transitions to StateDownloading if possible.
// Returns true to continue processing, false if a terminal state was reached.
func (p *Pipeline) processReady() bool {
	if p.canStartDownloading() {
		p.transitionTo(StateDownloading)
		return true
	}
	return true
}

// processDownloading handles the Downloading state.
// It executes the download function and transitions to StateIndexing if successful.
// Returns true to continue processing, false if a terminal state was reached.
// Returns an error if the download step fails.
// TODO: document expected error's once the Core logic is implemented
func (p *Pipeline) processDownloading(ctx context.Context) (bool, error) {
	p.logger.Debug().Msg("starting download step")

	if err := p.core.Download(ctx); err != nil {
		p.logger.Error().
			Err(err).
			Msg("download step failed")
		return false, err
	}

	p.logger.Debug().Msg("download step completed")

	// If we can't transition to indexing after successful download, cancel
	if !p.canStartIndexing() {
		p.transitionTo(StateCanceled)
		return false, nil
	}

	p.transitionTo(StateIndexing)
	return true, nil
}

// processIndexing handles the Indexing state.
// It executes the index function and transitions to StateWaitingPersist if possible.
// Returns true to continue processing, false if a terminal state was reached.
// Returns an error if the index step fails.
// TODO: document expected error's once the Core logic is implemented
func (p *Pipeline) processIndexing(ctx context.Context) (bool, error) {
	p.logger.Debug().Msg("starting index step")

	if err := p.core.Index(ctx); err != nil {
		p.logger.Error().
			Err(err).
			Msg("index step failed")
		return false, err
	}

	p.logger.Debug().Msg("index step completed")

	// If we can't transition to waiting for persist after successful indexing, cancel
	if !p.canWaitForPersist() {
		p.transitionTo(StateCanceled)
		return false, nil
	}

	p.transitionTo(StateWaitingPersist)
	return true, nil
}

// processWaitingPersist handles the WaitingPersist state.
// It checks if the conditions for persisting are met and transitions to StatePersisting if possible.
// Returns true to continue processing, false if a terminal state was reached.
func (p *Pipeline) processWaitingPersist() bool {
	if p.canStartPersisting() {
		p.transitionTo(StatePersisting)
		return true
	}

	// we need to wait for the result to be sealed and for the parent pipeline to finish persisting.
	// this method may be called multiple times before it's ready to transition to the persisting state.
	return true
}

// processPersisting handles the Persisting state.
// It executes the persist function and transitions to StateComplete if successful.
// Returns true to continue processing, false if a terminal state was reached.
// Returns an error if the persist step fails.
// TODO: document expected error's once the Core logic is implemented
func (p *Pipeline) processPersisting(ctx context.Context) (bool, error) {
	p.logger.Debug().Msg("starting persist step")

	if err := p.core.Persist(ctx); err != nil {
		p.logger.Error().
			Err(err).
			Msg("persist step failed")
		return false, err
	}

	p.logger.Debug().Msg("persist step completed")
	p.transitionTo(StateComplete)
	return false, nil
}

// processCancel handles the cancelation of the pipeline.
// It calls the core's Abort method to perform any necessary cleanup.
// Returns an error if the abort step fails.
// TODO: document expected error's once the Core logic is implemented
func (p *Pipeline) processCancel(ctx context.Context) error {
	p.cancel()
	return p.core.Abort(ctx)
}

// canStartDownloading checks if the pipeline can transition from Ready to Downloading.
//
// Conditions for transition:
//  1. The current state must be Ready
//  2. The parent pipeline must be in an active state (StateDownloading, StateIndexing,
//     StateWaitingPersist, StatePersisting, or StateComplete)
func (p *Pipeline) canStartDownloading() bool {
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
// 2. The parent pipeline must not be canceled
func (p *Pipeline) canStartIndexing() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.state == StateDownloading && p.parentState != StateCanceled
}

// canWaitForPersist checks if the pipeline can transition from Indexing to WaitingPersist.
//
// Conditions for transition:
// 1. The current state must be Indexing
// 2. The parent pipeline must not be canceled
func (p *Pipeline) canWaitForPersist() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.state == StateIndexing && p.parentState != StateCanceled
}

// canStartPersisting checks if the pipeline can transition from WaitingPersist to Persisting.
//
// Conditions for transition:
// 1. The current state must be WaitingPersist
// 2. The data must be sealed
// 3. The parent pipeline must be complete
func (p *Pipeline) canStartPersisting() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.state == StateWaitingPersist && p.isSealed && p.parentState == StateComplete
}
