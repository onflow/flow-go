package pipeline

import (
	"context"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
)

// State represents the state of the processing pipeline
type State int

const (
	//StateReady is the initial state after instantiation and before downloading has begun
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

// ProcessFunc is a function that processes a step in the pipeline
type ProcessFunc func(context.Context) error

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
	// InitialState sets the starting state of the pipeline (defaults to Ready if not specified)
	InitialState State
	// InitialDescendsFromSealed indicates if this pipeline initially descends from the sealed result
	InitialDescendsFromSealed bool
	// ChildrenStateUpdateChan is the channel to send state updates to children
	ChildrenStateUpdateChan chan<- StateUpdate

	downloadFunc ProcessFunc
	indexFunc    ProcessFunc
	persistFunc  ProcessFunc
}

// Pipeline represents a generic processing pipeline with state transitions.
// It processes data through sequential states: Ready -> Downloading -> Indexing ->
// WaitingPersist -> Persisting -> Complete, with conditions for each transition.
type Pipeline struct {
	logger             zerolog.Logger
	stateUpdateChan    chan StateUpdate
	childrenUpdateChan chan<- StateUpdate

	mu                 sync.RWMutex
	state              State
	isSealed           bool
	descendsFromSealed bool
	parentState        State

	downloadFunc ProcessFunc
	indexFunc    ProcessFunc
	persistFunc  ProcessFunc

	stateNotifier engine.Notifier
}

// NewPipeline creates a new processing pipeline.
// The pipeline is initialized in the Ready state and will begin processing
// when Run is called.
func NewPipeline(config Config) *Pipeline {
	// Set default initial state if not specified
	initialState := StateReady
	if config.InitialState != StateReady {
		initialState = config.InitialState
	}

	p := &Pipeline{
		//TODO: Should we have some unique ID for each pipeline, to identify their logs?
		logger:             config.Logger.With().Str("component", "pipeline").Logger(),
		stateUpdateChan:    make(chan StateUpdate, 10),
		childrenUpdateChan: config.ChildrenStateUpdateChan,
		state:              initialState,
		isSealed:           config.IsSealed,
		stateNotifier:      engine.NewNotifier(),

		downloadFunc: config.downloadFunc,
		indexFunc:    config.indexFunc,
		persistFunc:  config.persistFunc,
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
func (p *Pipeline) Run(ctx context.Context) {
	go p.listenForStateUpdates(ctx)

	notifierChan := p.stateNotifier.Channel()

	// Trigger initial check
	p.stateNotifier.Notify()

	for {
		select {
		case <-ctx.Done():
			p.transitionTo(StateCanceled)
			return
		case <-notifierChan:
			if processed := p.processCurrentState(ctx); !processed {
				// Terminal state reached
				return
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

// listenForStateUpdates listens for state updates from the parent pipeline
// and updates internal state accordingly. If the pipeline no longer descends
// from the latest persisted result, it transitions to the StateCanceled.
func (p *Pipeline) listenForStateUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-p.stateUpdateChan:
			p.mu.Lock()
			previousDescendsFromLatest := p.descendsFromSealed
			p.descendsFromSealed = update.DescendsFromLastPersistedSealed
			p.parentState = update.ParentState
			p.mu.Unlock()

			// If we no longer descend from latest, cancel the pipeline
			if previousDescendsFromLatest && !update.DescendsFromLastPersistedSealed {
				p.transitionTo(StateCanceled)
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
	if p.childrenUpdateChan == nil {
		return
	}

	p.mu.RLock()
	// Only descend from latest if this pipeline descends AND is not canceled
	descendsForChildren := p.descendsFromSealed && p.state != StateCanceled
	update := StateUpdate{
		DescendsFromLastPersistedSealed: descendsForChildren,
		ParentState:                     p.state,
	}
	p.mu.RUnlock()

	p.childrenUpdateChan <- update
}

// processCurrentState handles the current state and transitions to the next state if possible.
// It returns false when a terminal state is reached (StateComplete or StateCanceled), true otherwise.
func (p *Pipeline) processCurrentState(ctx context.Context) bool {
	p.mu.RLock()
	currentState := p.state
	p.mu.RUnlock()

	switch currentState {
	case StateReady:
		return p.processReady()
	case StateDownloading:
		return p.processDownloading(ctx)
	case StateIndexing:
		return p.processIndexing(ctx)
	case StateWaitingPersist:
		return p.processWaitingPersist()
	case StatePersisting:
		return p.processPersisting(ctx)
	case StateComplete, StateCanceled:
		// Terminal states
		return false
	default:
		p.logger.Error().Str("state", currentState.String()).Msg("invalid state")
		return false
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
		Msg("pipeline state transition")

	// Broadcast state update to children
	go p.broadcastStateUpdate()

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
// It executes the download function and transitions to StateIndexing if possible.
// Returns true to continue processing, false if a terminal state was reached.
//
// No error returns during normal operations.
func (p *Pipeline) processDownloading(ctx context.Context) bool {
	p.logger.Info().Msg("starting download step")

	if err := p.downloadFunc(ctx); err != nil {
		p.logger.Error().Err(err).Msg("download step failed")
		p.transitionTo(StateCanceled)
		return false
	}

	p.logger.Info().Msg("download step completed")

	if p.canStartIndexing() {
		p.transitionTo(StateIndexing)
		return true
	}

	return true
}

// processIndexing handles the Indexing state.
// It executes the index function and transitions to StateWaitingPersist if possible.
// Returns true to continue processing, false if a terminal state was reached.
//
// No error returns during normal operations.
func (p *Pipeline) processIndexing(ctx context.Context) bool {
	p.logger.Info().Msg("starting index step")

	if err := p.indexFunc(ctx); err != nil {
		p.logger.Error().Err(err).Msg("index step failed")
		p.transitionTo(StateCanceled)
		return false
	}

	p.logger.Info().Msg("index step completed")

	if p.canWaitForPersist() {
		p.transitionTo(StateWaitingPersist)
		return true
	}

	return true
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
//
// No error returns during normal operations.
func (p *Pipeline) processPersisting(ctx context.Context) bool {
	p.logger.Info().Msg("starting persist step")

	if err := p.persistFunc(ctx); err != nil {
		p.logger.Error().Err(err).Msg("persist step failed")
		p.transitionTo(StateCanceled)
		return false
	}

	p.logger.Info().Msg("persist step completed")
	p.transitionTo(StateComplete)
	return false
}

// canStartDownloading checks if the pipeline can transition from Ready to Downloading.
//
// Conditions for transition:
//  1. The pipeline must descend from the last persisted sealed result
//  2. The parent pipeline must be in an active state (StateDownloading, StateIndexing,
//     StateWaitingPersist, StatePersisting, or StateComplete)
func (p *Pipeline) canStartDownloading() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

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
// 1. The pipeline must descend from the last persisted sealed result
// 2. The parent pipeline must not be canceled
func (p *Pipeline) canStartIndexing() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.descendsFromSealed && p.parentState != StateCanceled
}

// canWaitForPersist checks if the pipeline can transition from Indexing to WaitingPersist.
//
// Conditions for transition:
// 1. The pipeline must descend from the last persisted sealed result
// 2. The parent pipeline must not be canceled
func (p *Pipeline) canWaitForPersist() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.descendsFromSealed && p.parentState != StateCanceled
}

// canStartPersisting checks if the pipeline can transition from WaitingPersist to Persisting.
//
// Conditions for transition:
// 1. The data must be sealed
// 2. The parent pipeline must be complete
func (p *Pipeline) canStartPersisting() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.isSealed && p.parentState == StateComplete
}
