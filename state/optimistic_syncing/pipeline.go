package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/qmuntal/stateless"
	"github.com/rs/zerolog"
)

const (
	StateReady          = "ready"
	StateDownloading    = "downloading"
	StateIndexing       = "indexing"
	StateWaitingPersist = "waiting_persist"
	StatePersisting     = "persisting"
	StateComplete       = "complete"
	StateCanceled       = "canceled"
)

const (
	TriggerStart            = "start"
	TriggerDownloadComplete = "download_complete"
	TriggerIndexingComplete = "indexing_complete"
	TriggerReadyToPersist   = "ready_to_persist"
	TriggerPersistComplete  = "persist_complete"
	TriggerCancel           = "cancel"
	triggerParentUpdate     = "parent_update"
)

type (
	OnDownload func() error
	OnIndex    func() error
	OnPersist  func() error

	StateListener func(newState string)
	Option        func(*Pipeline)
)

// ParentUpdate represents an update from a parent pipeline
type ParentUpdate struct {
	ParentState        string
	DescendsFromSealed bool
	IsSealed           bool
}

// Pipeline represents a pipeline for processing an execution result
type Pipeline struct {
	//TODO: Not sure if mutex needed, but the data could be changed outside, and this could be potentially in parallel with getters
	mutex        sync.RWMutex
	stateMachine *stateless.StateMachine
	logger       zerolog.Logger

	// Current state information
	parentState        string
	descendsFromSealed bool
	isSealed           bool

	// Operation callbacks
	download OnDownload
	index    OnIndex
	persist  OnPersist

	stateListener StateListener
}

// WithStateListener sets a listener for state changes
func WithStateListener(listener StateListener) Option {
	return func(p *Pipeline) {
		p.stateListener = listener
	}
}

// NewPipeline creates a new processing pipeline
func NewPipeline(
	logger zerolog.Logger,
	download OnDownload,
	index OnIndex,
	persist OnPersist,
	opts ...Option,
) *Pipeline {
	p := &Pipeline{
		logger:       logger.With().Str("component", "pipeline").Logger(),
		download:     download,
		index:        index,
		persist:      persist,
		stateMachine: stateless.NewStateMachine(StateReady),
	}

	for _, opt := range opts {
		opt(p)
	}

	p.pipelineInitialize()

	return p
}

// pipelineInitialize configures the state machine transitions and handlers
func (p *Pipeline) pipelineInitialize() {
	p.stateMachine.OnTransitioned(func(_ context.Context, tr stateless.Transition) {
		state, ok := tr.Destination.(string)
		if !ok {
			p.logger.Error().Msgf("invalid state type %s", state)
			return
		}

		if p.stateListener != nil {
			p.stateListener(state)
		}
	})

	// Configure ready state
	p.stateMachine.Configure(StateReady).
		Permit(TriggerStart, StateDownloading, func(_ context.Context, _ ...any) bool { return p.canStartDownloading() }).
		Permit(TriggerCancel, StateCanceled).
		InternalTransition(triggerParentUpdate, func(_ context.Context, _ ...any) error {
			p.Fire(TriggerStart)
			return nil
		})

	// Configure downloading state
	p.stateMachine.Configure(StateDownloading).
		Permit(TriggerDownloadComplete, StateIndexing, func(_ context.Context, _ ...any) bool { return p.canStartIndexing() }).
		Permit(TriggerCancel, StateCanceled).
		OnEntry(func(_ context.Context, args ...any) error {
			return p.onDownloadingEnter()
		})

	// Configure indexing state
	p.stateMachine.Configure(StateIndexing).
		Permit(TriggerIndexingComplete, StateWaitingPersist, func(_ context.Context, _ ...any) bool { return p.canWaitForPersist() }).
		Permit(TriggerCancel, StateCanceled).
		OnEntry(func(_ context.Context, args ...any) error {
			return p.onIndexingEnter()
		})

	// Configure waiting_persist state
	p.stateMachine.Configure(StateWaitingPersist).
		Permit(TriggerReadyToPersist, StatePersisting, func(_ context.Context, _ ...any) bool { return p.canStartPersisting() }).
		Permit(TriggerCancel, StateCanceled).
		InternalTransition(triggerParentUpdate, func(_ context.Context, _ ...any) error {
			p.Fire(TriggerReadyToPersist)
			return nil
		})

	// Configure persisting state
	p.stateMachine.Configure(StatePersisting).
		Permit(TriggerPersistComplete, StateComplete).
		Permit(TriggerCancel, StateCanceled).
		OnEntry(func(_ context.Context, args ...any) error {
			return p.onPersistingEnter()
		})

	// Configure complete state (final on success)
	p.stateMachine.Configure(StateComplete).
		OnEntry(func(_ context.Context, args ...any) error {
			return p.onCompleteEnter()
		})

	// Configure canceled state (final on failure)
	p.stateMachine.Configure(StateCanceled).
		OnEntry(func(_ context.Context, args ...any) error {
			return p.onCanceledEnter()
		})
}

func (p *Pipeline) canStartDownloading() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.descendsFromSealed &&
		(p.parentState == StateDownloading ||
			p.parentState == StateIndexing ||
			p.parentState == StateWaitingPersist ||
			p.parentState == StatePersisting ||
			p.parentState == StateComplete)
}

func (p *Pipeline) canStartIndexing() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.descendsFromSealed && p.parentState != StateCanceled
}

func (p *Pipeline) canWaitForPersist() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.descendsFromSealed && p.parentState != StateCanceled
}

func (p *Pipeline) canStartPersisting() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.isSealed && p.parentState == StateComplete
}

func (p *Pipeline) onDownloadingEnter() error {
	p.logger.Debug().Msg("pipeline entered downloading state, starting download")

	err := p.download()
	if err != nil {
		p.logger.Error().Err(err).Msg("download failed")
		p.Fire(TriggerCancel) // do not return error here, as this will stop download state transition
		return nil
	}

	p.logger.Debug().Msg("download completed successfully")
	p.Fire(TriggerDownloadComplete)
	return nil
}

func (p *Pipeline) onIndexingEnter() error {
	p.logger.Debug().Msg("pipeline entered indexing state, starting indexing")

	err := p.index()
	if err != nil {
		p.logger.Error().Err(err).Msg("indexing failed")
		p.Fire(TriggerCancel)
		return nil // do not return error here, as this will stop indexing state transition
	}

	p.logger.Debug().Msg("indexing completed successfully")
	p.Fire(TriggerIndexingComplete)
	return nil
}

func (p *Pipeline) onPersistingEnter() error {
	p.logger.Debug().Msg("pipeline entered persisting state, starting persist")

	err := p.persist()
	if err != nil {
		p.logger.Error().Err(err).Msg("persisting failed")
		p.Fire(TriggerCancel) // do not return error here, as this will stop persist state transition
		return nil
	}

	p.logger.Debug().Msg("persisting completed successfully")
	p.Fire(TriggerPersistComplete)
	return nil
}

// TODO: This is just stub. will be implemented later if needed
func (p *Pipeline) onCompleteEnter() error {
	p.logger.Info().Msg("pipeline processing completed successfully")
	return nil
}

// TODO: This is just stub. will be implemented later if needed
func (p *Pipeline) onCanceledEnter() error {
	p.logger.Info().Msg("pipeline processing was canceled")
	return nil
}

// UpdateFromParent processes a parent update
func (p *Pipeline) UpdateFromParent(update ParentUpdate) {
	p.mutex.Lock()
	previousDescendsFromSealed := p.descendsFromSealed

	p.parentState = update.ParentState
	p.descendsFromSealed = update.DescendsFromSealed
	p.isSealed = update.IsSealed
	p.mutex.Unlock()

	if previousDescendsFromSealed && !p.descendsFromSealed {
		p.logger.Info().Msg("no longer descends from sealed result, canceling")
		p.Fire(TriggerCancel)
	} else {
		// Otherwise just check if the update allows us to transition to internal state
		p.Fire(triggerParentUpdate)
	}
}

// Fire triggers a state transition
func (p *Pipeline) Fire(trigger string) {
	p.mutex.RLock()

	currentStateRaw := p.stateMachine.MustState()
	currentState, ok := currentStateRaw.(string)
	if !ok {
		p.logger.Error().
			Str("trigger", trigger).
			Str("raw_state", fmt.Sprintf("%v", currentStateRaw)).
			Msg("invalid state type, unable to process trigger")
		p.mutex.RUnlock()
		return
	}
	p.mutex.RUnlock()

	p.logger.Debug().
		Str("from_state", currentState).
		Str("trigger", trigger).
		Msg("attempting state transition")

	if err := p.stateMachine.Fire(trigger); err != nil {
		p.logger.Error().
			Err(err).
			Str("from_state", currentState).
			Str("trigger", trigger).
			Msg("failed to transition state")
	}
}

func (p *Pipeline) CurrentState() string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	currentStateRaw := p.stateMachine.MustState()
	currentState, ok := currentStateRaw.(string)
	if !ok {
		p.logger.Error().Msg("invalid state type")
		return ""
	}

	return currentState
}

func (p *Pipeline) IsDescendantOfSealed() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.descendsFromSealed
}

func (p *Pipeline) IsSealed() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.isSealed
}
