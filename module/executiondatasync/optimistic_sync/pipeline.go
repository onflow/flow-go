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

// Pipeline represents a processing pipelined state machine with the following sequence of states:
// 1. Pending
// 2. Ready
// 3. Downloading
// 4. Indexing
// 5. WaitingPersist
// 6. Persisting
// 7. Complete
//
// The state machine is initialized in the Pending state, and can transition to Abandoned at any time
// if the parent pipeline is abandoned.
//
// The state machine is designed to be run in a single goroutine, and is not safe for concurrent access.
// The Run method must only be called once.
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
// Pipelines must only be created for ExecutionResults that descend from the latest persisted sealed result.
// The pipeline is initialized in the Pending state.
//
// Parameters:
//   - log: the logger to use for the pipeline
//   - isSealed: indicates if the pipeline's ExecutionResult is sealed
//   - executionResult: processed execution result
//   - core: implements the processing logic for the pipeline
//   - statePublisher: called when the pipeline needs to broadcast state updates
//
// Returns:
//   - *PipelineImpl: the newly created pipeline
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
// Parameters:
//   - ctx: the context to use for process lifecycle
//   - core: the core implementation to use for processing
//
// Returns:
//   - error: any error that occurred during processing, including context cancellation
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are unexpected and potential indicators bugs or of corrupted internal state
//
// Concurrency safety:
//   - Not safe for concurrent access. Run must only be called once.
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

	// handle context cancellation
	// there are two cases:
	// 1. parent context was canceled. in this case, we should shutdown without transitioning
	//    to avoid cascading abandoned state updates since all pipelines may share the same
	//    root context
	// 2. pipeline's context was canceled. in this case, we should transition to abandoned
	//    state which communicates the update to all ancestors
	shutdown := func() {
		if parentCtx.Err() == nil && p.GetState() != StateAbandoned {
			p.transitionTo(StateAbandoned)
		}
	}

	for {
		select {
		case <-ctx.Done():
			shutdown()
			return ctx.Err()

		case <-notifierChan:
			processing, err := p.processCurrentState(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					shutdown()
				}
				return err
			}
			if !processing {
				return nil // terminal state reached
			}
		}
	}
}

// GetState returns the current state of the pipeline.
//
// Returns:
//   - State: the current state of the pipeline
//
// Concurrency safety:
//   - Safe for concurrent access
func (p *PipelineImpl) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// SetSealed marks the data as sealed, which enables transitioning from StateWaitingPersist to StatePersisting.
//
// Concurrency safety:
//   - Safe for concurrent access
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
//
// Parameters:
//   - parentState: the new state of the parent pipeline
//
// Concurrency safety:
//   - Safe for concurrent access
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

// setState sets the state of the pipeline and logs the transition.
//
// Parameters:
//   - newState: the new state to set
//
// Concurrency safety:
//   - Not safe for concurrent access.
func (p *PipelineImpl) setState(newState State) {
	oldState := p.state
	p.state = newState

	p.log.Debug().
		Str("old_state", oldState.String()).
		Str("new_state", newState.String()).
		Msg("pipeline state transition")
}

// processCurrentState handles the current state and transitions to the next state if possible.
//
// Parameters:
//   - ctx: the context to use for cancellation
//
// Returns:
//   - bool: true if processing should continue, false if a terminal state was reached
//   - error: any error that occurred during processing
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are unexpected and potential indicators bugs or of corrupted internal state
//
// Concurrency safety:
//   - Safe for concurrent access.
func (p *PipelineImpl) processCurrentState(ctx context.Context) (bool, error) {
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
	case StateAbandoned:
		return p.processAbandoned(ctx)
	case StateComplete:
		// Terminal state
		return false, nil
	default:
		return false, fmt.Errorf("invalid pipeline state: %s", currentState)
	}
}

// transitionTo transitions the pipeline to the given state and broadcasts
// the state change to children pipelines.
//
// Parameters:
//   - newState: the state to transition to
//
// Concurrency safety:
//   - Safe for concurrent access
func (p *PipelineImpl) transitionTo(newState State) {
	p.mu.Lock()
	p.mu.Unlock()
	p.setState(newState)

	p.statePublisher(newState)

	if newState == StateComplete {
		return
	}

	p.stateNotifier.Notify()
}

// processReady handles the Ready state and transitions to StateDownloading if possible.
//
// Returns:
//   - bool: true if processing should continue, false if a terminal state was reached
//
// Concurrency safety:
//   - Safe for concurrent access, but not intended to be called concurrently with other process methods.
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
// Parameters:
//   - ctx: the context to use for cancellation
//
// Returns:
//   - bool: true if processing should continue, false if a terminal state was reached
//   - error: any error that occurred during processing
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are unexpected and potential indicators bugs or of corrupted internal state
//
// Concurrency safety:
//   - Safe for concurrent access, but not intended to be called concurrently with other process methods.
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
		return false, nil
	}
	p.transitionTo(StateIndexing)
	return true, nil
}

// processIndexing handles the Indexing state.
// It executes the index function and transitions to StateWaitingPersist if possible.
//
// Parameters:
//   - ctx: the context to use for cancellation
//
// Returns:
//   - bool: true if processing should continue, false if a terminal state was reached
//   - error: any error that occurred during processing
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are unexpected and potential indicators bugs or of corrupted internal state
//
// Concurrency safety:
//   - Safe for concurrent access, but not intended to be called concurrently with other process methods.
func (p *PipelineImpl) processIndexing(ctx context.Context) (bool, error) {
	p.log.Debug().Msg("starting index step")

	if err := p.core.Index(ctx); err != nil {
		p.log.Error().Err(err).Msg("index step failed")
		return false, err
	}

	p.log.Debug().Msg("index step completed")

	if !p.canWaitForPersist() {
		// If we can't transition to waiting for persist after successful indexing, abandon
		p.transitionTo(StateAbandoned)
		return false, nil
	}

	p.transitionTo(StateWaitingPersist)
	return true, nil
}

// processWaitingPersist handles the WaitingPersist state.
// It checks if the conditions for persisting are met and transitions to StatePersisting if possible.
//
// Returns:
//   - bool: true if processing should continue, false if a terminal state was reached
//
// Concurrency safety:
//   - Safe for concurrent access, but not intended to be called concurrently with other process methods.
func (p *PipelineImpl) processWaitingPersist() bool {
	if p.canStartPersisting() {
		p.transitionTo(StatePersisting)
		return true
	}
	return true
}

// processPersisting handles the Persisting state.
// It executes the persist function and transitions to StateComplete if successful.
//
// Parameters:
//   - ctx: the context to use for cancellation
//
// Returns:
//   - bool: true if processing should continue, false if a terminal state was reached
//   - error: any error that occurred during processing
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are unexpected and potential indicators bugs or of corrupted internal state
//
// Concurrency safety:
//   - Safe for concurrent access, but not intended to be called concurrently with other process methods.
func (p *PipelineImpl) processPersisting(ctx context.Context) (bool, error) {
	p.log.Debug().Msg("starting persist step")

	if err := p.core.Persist(ctx); err != nil {
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
// Parameters:
//   - ctx: the context to use for cancellation
//
// Returns:
//   - bool: false to indicate terminal state reached
//   - error: any error that occurred during processing
//
// Expected Errors:
//   - context.Canceled: when the context is canceled
//   - All other errors are unexpected and potential indicators bugs or of corrupted internal state
//
// Concurrency safety:
//   - Safe for concurrent access, but not intended to be called concurrently with other process methods.
func (p *PipelineImpl) processAbandoned(ctx context.Context) (bool, error) {
	p.log.Debug().Msg("processing abandoned state")

	if err := p.core.Abandon(ctx); err != nil {
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
//
// Returns:
//   - bool: true if the pipeline can start downloading
//
// Concurrency safety:
//   - Safe for concurrent access
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
//
// Returns:
//   - bool: true if the pipeline can start indexing
//
// Concurrency safety:
//   - Safe for concurrent access
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
//
// Returns:
//   - bool: true if the pipeline can wait for persist
//
// Concurrency safety:
//   - Safe for concurrent access
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
//
// Returns:
//   - bool: true if the pipeline can start persisting
//
// Concurrency safety:
//   - Safe for concurrent access
func (p *PipelineImpl) canStartPersisting() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.state == StateWaitingPersist && p.isSealed && p.parentState == StateComplete
}
