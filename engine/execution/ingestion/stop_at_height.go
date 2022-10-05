package ingestion

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
)

type StopControl struct {
	sync.RWMutex
	height             uint64
	crash              bool
	commenced          bool // if stopping process has started. We disallow any changes then.
	set                bool // whether stop at height has been set at all, it its envisioned most of the time it won't
	paused             bool // if execution is paused
	stopAfterExecuting flow.Identifier
	log                zerolog.Logger
}

// NewStopControl creates new empty NewStopControl
func NewStopControl(log zerolog.Logger, paused bool) *StopControl {
	return &StopControl{
		paused:    paused,
		commenced: paused,
		log:       log,
	}
}

// Get returns
// boolean indicating if value is set - for easier comparisons, since its envisions most of the time this struct will be empty
// height and crash values
func (s *StopControl) Get() (bool, uint64, bool) {
	s.RLock()
	defer s.RUnlock()
	return s.set, s.height, s.crash
}

func (s *StopControl) IsPaused() bool {
	s.RLock()
	defer s.RUnlock()
	return s.paused
}

// Try runs function f with current values of height and crash if the values are set
// f should return true if it started a process of stopping, so no further changes will
// be accepted.
// Try returns whatever f returned, or false if f has not been called
func (s *StopControl) Try(f func(uint64, bool) bool) bool {
	s.Lock()
	defer s.Unlock()

	if !s.set {
		return false
	}

	commenced := f(s.height, s.crash)

	if commenced {
		s.commenced = true
	}

	return commenced
}

// Set sets new values and return old ones:
//   - set, whether values were previously set
//   - height
//   - crash
//
// Returns error is the stopping process has already commenced, new values will be rejected.
func (s *StopControl) Set(height uint64, crash bool) (bool, uint64, bool, error) {
	s.Lock()
	defer s.Unlock()

	oldSet := s.set
	oldHeight := s.height
	oldCrash := s.crash

	if s.commenced {
		return oldSet, oldHeight, oldCrash, fmt.Errorf("cannot update stop height, stopping already in progress for height %d with crash=%t", oldHeight, oldCrash)
	}

	s.set = true
	s.height = height
	s.crash = crash
	s.stopAfterExecuting = flow.ZeroID

	return oldSet, oldHeight, oldCrash, nil
}

func (s *StopControl) BlockProcessable(b *flow.Header) bool {

	s.RLock()
	defer s.RUnlock()

	if !s.set {
		return false
	}

	if s.paused {
		return true
	}

	// skips blocks at or above requested stop height
	if b.Height >= s.height {
		s.log.Warn().Msgf("Skipping execution of %s at height %d because stop has been requested at height %d", b.ID(), b.Height, s.height)
		return true
	}

	return false
}

func (s *StopControl) BlockFinalized(ctx context.Context, execState state.ReadOnlyExecutionState, h *flow.Header) {

	s.Lock()
	defer s.Unlock()

	if s.paused || !s.set {
		return
	}

	// Once finalization reached stop height we can be sure no other fork will be valid at this height,
	// if this block's parent has been executed, we are safe to stop or crash.
	// This will happen during normal execution, where blocks are executed before they are finalized.
	// However, it is possible that EN block computation progress can fall behind. In this case,
	// we want to crash only after the execution reached the stop height.
	if h.Height == s.height {

		executed, err := state.IsBlockExecuted(ctx, execState, h.ParentID)
		if err != nil {
			// any error here would indicate unexpected storage error, so we crash the node
			s.log.Fatal().Err(err).Str("block_id", h.ID().String()).Msg("failed to check if the block has been executed")
			return
		}

		if executed {
			s.stopExecution()
		} else {
			s.stopAfterExecuting = h.ParentID
		}

	}

}

func (s *StopControl) BlockExecuted(h *flow.Header) {
	s.Lock()
	defer s.Unlock()

	if !s.set {
		return
	}

	if s.stopAfterExecuting == h.ID() {
		// double check. Even if requested stop height has been changed multiple times,
		// as long as it matches this block we are safe to terminate

		if h.Height == s.height-1 {
			s.stopExecution()
		}
	}
}

func (s *StopControl) stopExecution() {
	if s.crash {
		s.log.Fatal().Msgf("Crashing as finalization reached requested stop height %d", s.height)
	} else {
		s.paused = true
		s.log.Warn().Msgf("Pausing execution as finalization reached requested stop height %d", s.height)
	}
}
