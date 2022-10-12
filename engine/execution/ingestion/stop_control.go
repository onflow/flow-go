package ingestion

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
)

// StopControl is a specialized component used by ingestion.Engine to encapsulate
// control of pausing/stopping blocks execution.
// It is intended to work tightly with the Engine, not as a general mechanism or interface.
// StopControl follows states described in StopState
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

type stopControlState byte

const (
	// StopControlOff default state, envisioned to be used most of the time. Stopping module is simply off,
	// blocks will be processed "as usual"
	StopControlOff stopControlState = iota

	// StopControlSet means stop height is set but not reached yet, and nothing related to stopping happened yet.
	// We can still go back to StopControlOff or progress to StopControlCommenced.
	StopControlSet

	// StopControlCommenced indicates that stopping process has commenced and no parameters can be changed anymore.
	// For example, blocks at or above stop height has been received, but finalization didn't reach stop height yet
	StopControlCommenced

	// StopControlPaused means EN has stopped processing blocks. It can happen by reaching stop height
	StopControlPaused
)

// NewStopControl creates new empty NewStopControl
func NewStopControl(log zerolog.Logger, paused bool) *StopControl {
	return &StopControl{
		paused:    paused,
		commenced: paused,
		log:       log,
	}
}

// IsPaused returns true is block execution has been paused
func (s *StopControl) IsPaused() bool {
	s.RLock()
	defer s.RUnlock()
	return s.paused
}

// SetStopHeight sets new stop height and crash mode, and return old values:
//   - set, whether values were previously set
//   - height
//   - crash
//
// Returns error is the stopping process has already commenced, new values will be rejected.
func (s *StopControl) SetStopHeight(height uint64, crash bool) (bool, uint64, bool, error) {
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

// GetStopHeight returns:
//   - set, whether values are set
//   - height
//   - crash
func (s *StopControl) GetStopHeight() (bool, uint64, bool) {
	s.RLock()
	defer s.RUnlock()

	return s.set, s.height, s.crash
}

// BlockProcessable should be called when new block is processable.
// It returns boolean indicating if processing should skip this block.
func (s *StopControl) BlockProcessable(b *flow.Header) bool {

	s.Lock()
	defer s.Unlock()

	if !s.set {
		return false
	}

	if s.paused {
		return true
	}

	// skips blocks at or above requested stop height
	if b.Height >= s.height {
		s.log.Warn().Msgf("Skipping execution of %s at height %d because stop has been requested at height %d", b.ID(), b.Height, s.height)
		s.commenced = true
		return true
	}

	return false
}

// BlockFinalized should be called when a block is marked as finalized
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
			s.set = true
		}

	}

}

// BlockExecuted should be called after a block has finished execution
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
		s.commenced = true
		s.log.Warn().Msgf("Pausing execution as finalization reached requested stop height %d", s.height)
	}
}
