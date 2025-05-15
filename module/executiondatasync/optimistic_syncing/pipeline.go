// Package pipeline provides the implementation for processing individual execution results
// through a multi-stage state machine. Each pipeline manages the lifecycle of downloading,
// indexing, and persisting execution data for a single ExecutionResult.
//
// The pipeline implements the state machine described in the Optimistic Syncing design,
// transitioning through states: StateReady -> StateDownloading -> StateIndexing -> StateWaitingPersist ->
// StatePersisting -> StateComplete, with an optional StateCanceled state for abandoned forks.
//
// Pipelines form a tree that mirrors the execution result tree. Each pipeline tracks
// its parent and communicates state updates to its children. This allows the entire
// tree to coordinate processing and handle chain reorganizations gracefully.
//
// State Transition Conditions:
//
//	Ready -> Downloading:
//	  - Must descend from last persisted sealed result
//	  - Parent must be in active state (Downloading/Indexing/WaitingPersist/Persisting/Complete)
//
//	Downloading -> Indexing:
//	  - Download completed successfully
//	  - Must still descend from sealed chain
//	  - Parent must not be canceled
//
//	Indexing -> WaitingPersist:
//	  - Indexing completed successfully
//	  - Must still descend from sealed chain
//	  - Parent must not be canceled
//
//	WaitingPersist -> Persisting:
//	  - ExecutionResult must be sealed
//	  - Parent pipeline must be complete
//
//	Persisting -> Complete:
//	  - Persist completed successfully
//
//	Any -> Canceled:
//	  - No longer descends from sealed chain
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
)

// State represents the state of the processing pipeline
type State int

const (
	// StateReady is the initial state after instantiation and before downloading has begun
	StateReady State = iota
	// StateDownloading represents the state where data download is in progress
	StateDownloading
	// StateIndexing represents the state where data is being indexed
	StateIndexing
	// StateWaitingPersist represents the state where all data is indexed, but conditions to persist are not met
	StateWaitingPersist
	// StatePersisting represents the state where the indexed data is being persisted to storage
	StatePersisting
	// StateComplete represents the state where all data is persisted to storage
	StateComplete
	// StateCanceled represents the state where processing was aborted
	StateCanceled
)

// String representation of states for logging
func (s State) String() string {
	switch s {
	case StateReady:
		return "ready"
	case StateDownloading:
		return "downloading"
	case StateIndexing:
		return "indexing"
	case StateWaitingPersist:
		return "waiting_persist"
	case StatePersisting:
		return "persisting"
	case StateComplete:
		return "complete"
	case StateCanceled:
		return "canceled"
	default:
		return ""
	}
}

// StateUpdate contains state update information
type StateUpdate struct {
	// DescendsFromSealed indicates if this pipeline descends from
	// the last persisted sealed result
	DescendsFromSealed bool
	// ParentState contains the state information from the parent pipeline
	ParentState State
}

// StateUpdatePublisher is a function that publishes state updates
type StateUpdatePublisher func(update StateUpdate)

// Pipeline represents a generic processing pipeline with state transitions.
// It processes data through sequential states: Ready -> Downloading -> Indexing ->
// WaitingPersist -> Persisting -> Complete, with conditions for each transition.
type Pipeline struct {
	logger         zerolog.Logger
	statePublisher StateUpdatePublisher

	mu                 sync.RWMutex
	state              State
	isSealed           bool
	descendsFromSealed bool
	parentState        State
	executionResult    *flow.ExecutionResult

	core          Core
	stateNotifier engine.Notifier
	cancel        context.CancelCauseFunc
}

// NewPipeline creates a new processing pipeline.
// Pipelines must only be created for ExecutionResults that descend from the latest persisted sealed result.
// The pipeline is initialized in the Ready state.
//
// Parameters:
//   - logger: the logger to use for the pipeline
//   - isSealed: indicates if the pipeline's ExecutionResult is sealed
//   - executionResult: processed execution result
//   - core: implements the processing logic for the pipeline
//   - stateUpdatePublisher: called when the pipeline needs to broadcast state updates
//
// Returns:
//   - new pipeline object
func NewPipeline(
	logger zerolog.Logger,
	isSealed bool,
	executionResult *flow.ExecutionResult,
	core Core,
	stateUpdatePublisher StateUpdatePublisher,
) *Pipeline {
	p := &Pipeline{
		logger:             logger.With().Str("component", "pipeline").Str("execution_result_id", executionResult.ExecutionDataID.String()).Str("block_id", executionResult.BlockID.String()).Logger(),
		statePublisher:     stateUpdatePublisher,
		state:              StateReady,
		isSealed:           isSealed,
		descendsFromSealed: true,
		stateNotifier:      engine.NewNotifier(),
		core:               core,
		executionResult:    executionResult,
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
func (p *Pipeline) Run(parentCtx context.Context) error {
	ctx, cancel := context.WithCancelCause(parentCtx)
	defer cancel(nil)

	p.mu.Lock()
	p.cancel = cancel
	p.mu.Unlock()

	notifierChan := p.stateNotifier.Channel()

	// Trigger initial check
	p.stateNotifier.Notify()

	for {
		select {
		case <-parentCtx.Done():
			return nil
		case <-ctx.Done():
			cause := context.Cause(ctx)
			if cause != nil && !errors.Is(cause, context.Canceled) {
				return cause
			}
			return nil
		case <-notifierChan:
			processed, err := p.processCurrentState(ctx)
			if err != nil {
				isContextCanceled := errors.Is(err, context.Canceled)
				isCtxCanceledWithSpecificCause := context.Cause(ctx) != nil && !errors.Is(context.Cause(ctx), context.Canceled)
				isParentCtxCanceledWithSpecificCause := context.Cause(parentCtx) != nil && !errors.Is(context.Cause(parentCtx), context.Canceled)

				contextCanceledGracefully :=
					isContextCanceled &&
						!isCtxCanceledWithSpecificCause &&
						!isParentCtxCanceledWithSpecificCause

				if contextCanceledGracefully {
					return nil
				}

				return err
			}
			if !processed {
				// Terminal state reached
				return nil
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

	// Trigger state check
	p.stateNotifier.Notify()
}

// UpdateState updates the pipeline's state based on the provided state update from its parent
// pipeline or the Results Forest.
//
// This function is the primary mechanism for state communication in the processing graph. State
// updates flow from parent to child pipelines, allowing coordinated lifecycle management across
// the entire execution result tree.
func (p *Pipeline) UpdateState(update StateUpdate) {
	if shouldAbort := p.handleStateUpdate(update); !shouldAbort {
		// Trigger state check
		p.stateNotifier.Notify()
		return
	}

	// If we no longer descend from the latest, cancel the pipeline
	p.transitionTo(StateCanceled)

	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cancel != nil {
		p.cancel(fmt.Errorf("abandoning due to parent updates"))
	}
}

// handleStateUpdate updates the internal state and returns whether the pipeline
// should be abandoned.
func (p *Pipeline) handleStateUpdate(update StateUpdate) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	previousDescendsFromSealed := p.descendsFromSealed
	p.descendsFromSealed = update.DescendsFromSealed
	p.parentState = update.ParentState

	return previousDescendsFromSealed && !update.DescendsFromSealed
}

// broadcastStateUpdate sends a state update via the state publisher.
func (p *Pipeline) broadcastStateUpdate() {
	if p.statePublisher == nil {
		return
	}

	p.mu.RLock()
	update := StateUpdate{
		DescendsFromSealed: p.descendsFromSealed,
		ParentState:        p.state,
	}
	p.mu.RUnlock()

	p.statePublisher(update)
}

// processCurrentState handles the current state and transitions to the next state if possible.
// It returns false when a terminal state is reached (StateComplete or StateCanceled), true otherwise.
// Returns an error if any processing step fails.
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
	case StateComplete, StateCanceled:
		// Terminal states
		return false, nil
	default:
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
func (p *Pipeline) processDownloading(ctx context.Context) (bool, error) {
	p.logger.Debug().Msg("starting download step")

	if err := p.core.Download(ctx); err != nil {
		p.logger.Error().
			Err(err).
			Msg("download step failed")
		return false, err
	}

	p.logger.Debug().Msg("download step completed")

	if !p.canStartIndexing() {
		// If we can't transition to indexing after successful download, cancel
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
func (p *Pipeline) processIndexing(ctx context.Context) (bool, error) {
	p.logger.Debug().Msg("starting index step")

	if err := p.core.Index(ctx); err != nil {
		p.logger.Error().
			Err(err).
			Msg("index step failed")
		return false, err
	}

	p.logger.Debug().Msg("index step completed")

	if !p.canWaitForPersist() {
		// If we can't transition to waiting for persist after successful indexing, cancel
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
	return true
}

// processPersisting handles the Persisting state.
// It executes the persist function and transitions to StateComplete if successful.
// Returns true to continue processing, false if a terminal state was reached.
// Returns an error if the persist step fails.
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

// canStartDownloading checks if the pipeline can transition from Ready to Downloading.
//
// Conditions for transition:
//  1. The current state must be Ready
//  2. The pipeline must descend from the last persisted sealed result
//  3. The parent pipeline must be in an active state (StateDownloading, StateIndexing,
//     StateWaitingPersist, StatePersisting, or StateComplete)
func (p *Pipeline) canStartDownloading() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.state != StateReady {
		return false
	}

	if !p.descendsFromSealed {
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
// 2. The pipeline must descend from the last persisted sealed result
// 3. The parent pipeline must not be canceled
func (p *Pipeline) canStartIndexing() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.state == StateDownloading && p.descendsFromSealed && p.parentState != StateCanceled
}

// canWaitForPersist checks if the pipeline can transition from Indexing to WaitingPersist.
//
// Conditions for transition:
// 1. The current state must be Indexing
// 2. The pipeline must descend from the last persisted sealed result
// 3. The parent pipeline must not be canceled
func (p *Pipeline) canWaitForPersist() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.state == StateIndexing && p.descendsFromSealed && p.parentState != StateCanceled
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
