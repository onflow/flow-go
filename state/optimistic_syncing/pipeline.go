package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
)

var (
	// ErrInvalidState indicates the pipeline is in an invalid state
	ErrInvalidState = errors.New("invalid pipeline state")
	// ErrPipelineCanceled indicates the pipeline was canceled
	ErrPipelineCanceled = errors.New("pipeline canceled")
)

// State represents the state of the processing pipeline
type State int

const (
	// StateReady is the initial state after instantiation and before downloading has begun
	StateReady State = iota
	//StateDownloading represents the state where data download is in progress
	StateDownloading
	//StateIndexing represents the state where data is being indexed
	StateIndexing
	//StateWaitingPersist represents the state where all data is indexed, but conditions to persist are not met
	StateWaitingPersist
	//StatePersisting represents the state where the indexed data is being persisted to storage
	StatePersisting
	//StateComplete represents the state where all prior steps completed successfully
	StateComplete
	//StateCanceled represents the state where processing was aborted
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
	// DescendsFromLastPersistedSealed indicates if this pipeline descends from
	// the last persisted result
	DescendsFromLastPersistedSealed bool
	// ParentState contains the state information from the parent pipeline
	ParentState State
}

// Config contains the configuration for the pipeline
type Config struct {
	// Logger is the logger to use for the pipeline
	Logger zerolog.Logger
	// IsSealed indicates if the data is sealed
	IsSealed bool
	// ExecutionResultID is a unique identifier for the execution result being processed
	ExecutionResultID string
	// BlockID is the identifier of the block being processed
	BlockID string
	// Core implements the processing logic for the pipeline
	Core Core
	// ChildrenStateUpdateChans are channels to send state updates to children
	ChildrenStateUpdateChans []chan<- StateUpdate
}

// Pipeline represents a generic processing pipeline with state transitions.
// It processes data through sequential states: Ready -> Downloading -> Indexing ->
// WaitingPersist -> Persisting -> Complete, with conditions for each transition.
type Pipeline struct {
	logger              zerolog.Logger
	stateUpdateChan     chan StateUpdate
	childrenUpdateChans []chan<- StateUpdate

	mu                 sync.RWMutex
	state              State
	isSealed           bool
	descendsFromSealed bool
	parentState        State
	executionResultID  string
	blockID            string

	core          Core
	stateNotifier engine.Notifier
}

// NewPipeline creates a new processing pipeline.
// The pipeline is always initialized in the Ready state and will begin processing
// when Run is called.
func NewPipeline(config Config) *Pipeline {
	p := &Pipeline{
		logger:              config.Logger.With().Str("component", "pipeline").Str("executionResultID", config.ExecutionResultID).Logger(),
		stateUpdateChan:     make(chan StateUpdate, 10),
		childrenUpdateChans: config.ChildrenStateUpdateChans,
		state:               StateReady,
		isSealed:            config.IsSealed,
		descendsFromSealed:  true,
		stateNotifier:       engine.NewNotifier(),
		core:                config.Core,
		executionResultID:   config.ExecutionResultID,
		blockID:             config.BlockID,
	}

	return p
}

// Run starts the pipeline processing and blocks until completion or context cancellation.
//
// This function handles the progression through the pipeline states, executing the appropriate
// processing functions at each step. It also listens for state updates from the parent pipeline
// and cancels processing if the pipeline no longer descends from the latest persisted result.
//
// When the pipeline reaches a terminal state (StateComplete or StateCanceled), the function returns.
// The function will also return if the provided context is canceled.
//
// Returns an error if any processing step fails or if the context is canceled.
func (p *Pipeline) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go p.listenForStateUpdates(ctx, cancel)

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
				// Terminal state reached
				if p.GetState() == StateCanceled {
					return ErrPipelineCanceled
				}
				return nil
			}
		}
	}
}

// GetStateUpdateChan returns the channel for receiving state updates for this pipeline.
// This channel can be used to send updates to this pipeline.
func (p *Pipeline) GetStateUpdateChan() chan StateUpdate {
	return p.stateUpdateChan
}

// GetState returns the current state of the pipeline.
func (p *Pipeline) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// SetSealed marks the data as sealed, which enable transitioning from StateWaitingPersist to StatePersisting.
func (p *Pipeline) SetSealed() {
	p.mu.Lock()
	p.isSealed = true
	p.mu.Unlock()

	// Trigger state check
	p.stateNotifier.Notify()
}

// updateState updates the internal state from a parent update and returns whether
// the pipeline should be abandoned (no longer descends from latest sealed)
func (p *Pipeline) updateState(update StateUpdate) bool {
	p.mu.Lock()
	previousDescendsFromLatest := p.descendsFromSealed
	p.descendsFromSealed = update.DescendsFromLastPersistedSealed
	p.parentState = update.ParentState
	p.mu.Unlock()

	return previousDescendsFromLatest && !update.DescendsFromLastPersistedSealed
}

// listenForStateUpdates listens for state updates from the parent pipeline
// and updates internal state accordingly. If the pipeline no longer descends
// from the latest persisted result, it transitions to the StateCanceled.
func (p *Pipeline) listenForStateUpdates(ctx context.Context, cancelFunc context.CancelFunc) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-p.stateUpdateChan:
			if shouldAbandon := p.updateState(update); shouldAbandon {
				p.broadcastStateUpdate()
				cancelFunc()
			} else {
				// Trigger state check
				p.stateNotifier.Notify()
			}
		}
	}
}

// broadcastStateUpdate sends a state update to all children pipelines.
// The update includes the current pipeline state and whether it descends
// from the latest persisted result.
func (p *Pipeline) broadcastStateUpdate() {
	if len(p.childrenUpdateChans) == 0 {
		return
	}

	p.mu.RLock()
	update := StateUpdate{
		DescendsFromLastPersistedSealed: p.descendsFromSealed,
		ParentState:                     p.state,
	}
	channels := p.childrenUpdateChans
	p.mu.RUnlock()

	for _, ch := range channels {
		ch <- update
	}
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
		return false, fmt.Errorf("%w: %s", ErrInvalidState, currentState.String())
	}
}

// transitionTo transitions the pipeline to the given state and broadcasts
// the state change to children pipelines.
func (p *Pipeline) transitionTo(newState State) {
	p.mu.Lock()
	oldState := p.state
	p.state = newState
	p.mu.Unlock()

	p.logger.Info().
		Str("oldState", oldState.String()).
		Str("newState", newState.String()).
		Str("executionResultID", p.executionResultID).
		Str("blockID", p.blockID).
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
	p.logger.Info().
		Str("executionResultID", p.executionResultID).
		Str("blockID", p.blockID).
		Msg("starting download step")

	if err := p.core.Download(ctx); err != nil {
		p.logger.Error().
			Err(err).
			Str("executionResultID", p.executionResultID).
			Str("blockID", p.blockID).
			Msg("download step failed")
		return false, err
	}

	p.logger.Info().
		Str("executionResultID", p.executionResultID).
		Str("blockID", p.blockID).
		Msg("download step completed")

	if p.canStartIndexing() {
		p.transitionTo(StateIndexing)
		return true, nil
	}

	// If we can't transition to indexing after successful download, cancel
	p.transitionTo(StateCanceled)
	return false, nil
}

// processIndexing handles the Indexing state.
// It executes the index function and transitions to StateWaitingPersist if possible.
// Returns true to continue processing, false if a terminal state was reached.
// Returns an error if the index step fails.
func (p *Pipeline) processIndexing(ctx context.Context) (bool, error) {
	p.logger.Info().
		Str("executionResultID", p.executionResultID).
		Str("blockID", p.blockID).
		Msg("starting index step")

	if err := p.core.Index(ctx); err != nil {
		p.logger.Error().
			Err(err).
			Str("executionResultID", p.executionResultID).
			Str("blockID", p.blockID).
			Msg("index step failed")
		return false, err
	}

	p.logger.Info().
		Str("executionResultID", p.executionResultID).
		Str("blockID", p.blockID).
		Msg("index step completed")

	if p.canWaitForPersist() {
		p.transitionTo(StateWaitingPersist)
		return true, nil
	}

	// If we can't transition to waiting for persist after successful indexing, cancel
	p.transitionTo(StateCanceled)
	return false, nil
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
	p.logger.Info().
		Str("executionResultID", p.executionResultID).
		Str("blockID", p.blockID).
		Msg("starting persist step")

	if err := p.core.Persist(ctx); err != nil {
		p.logger.Error().
			Err(err).
			Str("executionResultID", p.executionResultID).
			Str("blockID", p.blockID).
			Msg("persist step failed")
		return false, err
	}

	p.logger.Info().
		Str("executionResultID", p.executionResultID).
		Str("blockID", p.blockID).
		Msg("persist step completed")
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
