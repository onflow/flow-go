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
	stopAfterExecuting flow.Identifier
	log                zerolog.Logger
	state              StopControlState
	lastExecutedHeight uint64 // last executed height, conveniently defaults to zero
}

type StopControlState byte

const (
	// StopControlOff default state, envisioned to be used most of the time. Stopping module is simply off,
	// blocks will be processed "as usual".
	StopControlOff StopControlState = iota

	// StopControlSet means stop height is set but not reached yet, and nothing related to stopping happened yet.
	// We can still go back to StopControlOff or progress to StopControlCommenced.
	StopControlSet

	// StopControlCommenced indicates that stopping process has commenced and no parameters can be changed anymore.
	// For example, blocks at or above stop height has been received, but finalization didn't reach stop height yet.
	// It can only progress to StopControlPaused
	StopControlCommenced

	// StopControlPaused means EN has stopped processing blocks. It can happen by reaching set stop height, or
	// if the node was started with a pause mode.
	// It is a final state and cannot be changed
	StopControlPaused
)

// NewStopControl creates new empty NewStopControl
func NewStopControl(log zerolog.Logger, paused bool) *StopControl {
	state := StopControlOff
	if paused {
		state = StopControlPaused
	}
	log.Debug().Msgf("created StopControl module with paused = %t", paused)
	return &StopControl{
		log:   log,
		state: state,
	}
}

// GetState returns current state of StopControl module
func (s *StopControl) GetState() StopControlState {
	return s.state
}

// IsPaused returns true is block execution has been paused
func (s *StopControl) IsPaused() bool {
	s.RLock()
	defer s.RUnlock()
	return s.state == StopControlPaused
}

// SetStopHeight sets new stop height and crash mode, and return old values:
//   - set, whether values were previously set
//   - height
//   - crash
//
// Returns error is the stopping process has already commenced, new values will be rejected.
func (s *StopControl) SetStopHeight(height uint64, crash bool) (uint64, bool, error) {
	s.Lock()
	defer s.Unlock()

	oldHeight := s.height
	oldCrash := s.crash

	if s.state == StopControlCommenced {
		return oldHeight, oldCrash, fmt.Errorf("cannot update stop height, stopping commenced for height %d with crash=%t", oldHeight, oldCrash)
	}

	if s.state == StopControlPaused {
		return oldHeight, oldCrash, fmt.Errorf("cannot update stop height, already paused")
	}

	if s.lastExecutedHeight >= height {
		return oldHeight, oldCrash, fmt.Errorf("cannot update stop height, given height %d at or below last executed %d", height, s.lastExecutedHeight)
	}

	s.log.Info().
		Uint64("height", height).Bool("crash", crash).
		Uint64("old_height", oldHeight).Bool("old_crash", oldCrash).Msg("new stop height set")

	s.log.Debug().Int8("previous_state", int8(s.state)).Int8("new_state", int8(StopControlSet)).Msg("StopControl state transition")
	s.state = StopControlSet

	s.height = height
	s.crash = crash
	s.stopAfterExecuting = flow.ZeroID

	return oldHeight, oldCrash, nil
}

// GetStopHeight returns:
//   - height
//   - crash
//
// Values are undefined if they were not previously set
func (s *StopControl) GetStopHeight() (uint64, bool) {
	s.RLock()
	defer s.RUnlock()

	return s.height, s.crash
}

// BlockProcessable should be called when new block is processable.
// It returns boolean indicating if the block should be processed.
func (s *StopControl) BlockProcessable(b *flow.Header) bool {

	s.Lock()
	defer s.Unlock()

	if s.state == StopControlOff {
		return true
	}

	if s.state == StopControlPaused {
		return false
	}

	// skips blocks at or above requested stop height
	if b.Height >= s.height {
		s.log.Warn().Msgf("Skipping execution of %s at height %d because stop has been requested at height %d", b.ID(), b.Height, s.height)
		s.log.Debug().Int8("previous_state", int8(s.state)).Int8("new_state", int8(StopControlCommenced)).Msg("StopControl state transition")
		s.state = StopControlCommenced // if block was skipped, move into commenced state
		return false
	}

	return true
}

// BlockFinalized should be called when a block is marked as finalized
func (s *StopControl) BlockFinalized(ctx context.Context, execState state.ReadOnlyExecutionState, h *flow.Header) {

	s.Lock()
	defer s.Unlock()

	if s.state == StopControlOff || s.state == StopControlPaused {
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

// BlockExecuted should be called after a block has finished execution
func (s *StopControl) BlockExecuted(h *flow.Header) {
	s.Lock()
	defer s.Unlock()

	if s.state == StopControlPaused {
		return
	}

	if h.Height > s.lastExecutedHeight {
		s.lastExecutedHeight = h.Height
	}

	if s.state == StopControlOff {
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
		s.log.Debug().Int8("previous_state", int8(s.state)).Int8("new_state", int8(StopControlPaused)).Msg("StopControl state transition")
		s.state = StopControlPaused
		s.log.Warn().Msgf("Pausing execution as finalization reached requested stop height %d", s.height)
	}
}
