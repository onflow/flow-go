package ingestion

import (
	"context"
	"fmt"
	"strings"
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
	log zerolog.Logger

	stopBoundary *StopBoundary

	// This is the block ID of the block that should be executed last.
	stopAfterExecuting flow.Identifier

	state StopControlState
}

type StopControlState string

const (
	// StopControlOff default state, envisioned to be used most of the time.
	// Stopping module is simply off, blocks will be processed "as usual".
	StopControlOff StopControlState = "Off"

	// StopControlSet means stopHeight is set but not reached yet,
	// and nothing related to stopping happened yet.
	// We could still go back to StopControlOff or progress to StopControlCommenced.
	StopControlSet StopControlState = "Set"

	// StopControlStopping indicates that stopping process has commenced
	// and no parameters can be changed.
	// For example, blocks at or above stopHeight has been received,
	// but finalization didn't reach stopHeight yet.
	// It can only progress to StopControlStopped
	StopControlStopping StopControlState = "Stopping"

	// StopControlStopped means EN has stopped processing blocks.
	// It can happen by reaching the set stopping `stopHeight`, or
	// if the node was started in pause mode.
	// It is a final state and cannot be changed
	StopControlStopped StopControlState = "Stopped"
)

type StopBoundary struct {
	// desired stopHeight, the first value new version should be used,
	// so this height WON'T be executed
	StopHeight uint64

	// if the node should crash or just pause after reaching stopHeight
	ShouldCrash bool
}

// String returns string in the format "crash@20023"
func (s *StopBoundary) String() string {
	if s == nil {
		return "none"
	}

	sb := strings.Builder{}
	if s.ShouldCrash {
		sb.WriteString("crash")
	} else {
		sb.WriteString("pause")
	}
	sb.WriteString("@")
	sb.WriteString(fmt.Sprintf("%d", s.StopHeight))

	return sb.String()
}

// NewStopControl creates new empty NewStopControl
func NewStopControl(
	log zerolog.Logger,
) *StopControl {
	log = log.With().Str("component", "stop_control").Logger()
	log.Debug().Msgf("Created")

	return &StopControl{
		log:   log,
		state: StopControlOff,
	}
}

// IsExecutionPaused returns true is block execution has been paused
func (s *StopControl) IsExecutionPaused() bool {
	st := s.getState()
	return st == StopControlStopped
}

// PauseExecution sets the state to StopControlPaused
func (s *StopControl) PauseExecution() {
	s.Lock()
	defer s.Unlock()

	s.setState(StopControlStopped)
}

// getState returns current state of StopControl module
func (s *StopControl) getState() StopControlState {
	s.RLock()
	defer s.RUnlock()

	return s.state
}

func (s *StopControl) setState(newState StopControlState) {
	if newState == s.state {
		return
	}

	s.log.Info().
		Str("from", string(s.state)).
		Str("to", string(newState)).
		Msg("State transition")

	s.state = newState
}

// SetStopHeight sets new stopHeight and shouldCrash mode.
// Returns error if the stopping process has already commenced.
func (s *StopControl) SetStopHeight(
	height uint64,
	crash bool,
) error {
	s.Lock()
	defer s.Unlock()

	if s.stopBoundary != nil && s.state == StopControlStopping {
		return fmt.Errorf(
			"cannot update stopHeight, "+
				"stopping commenced for %s",
			s.stopBoundary,
		)
	}

	if s.state == StopControlStopped {
		return fmt.Errorf("cannot update stopHeight, already paused")
	}

	stopBoundary := &StopBoundary{
		StopHeight:  height,
		ShouldCrash: crash,
	}

	s.log.Info().
		Stringer("old_stop", s.stopBoundary).
		Stringer("new_stop", stopBoundary).
		Msg("new stopHeight set")

	s.setState(StopControlSet)
	s.stopBoundary = stopBoundary
	s.stopAfterExecuting = flow.ZeroID

	return nil
}

// GetNextStop returns the first upcoming stop boundary values are undefined
// if they were not previously set.
func (s *StopControl) GetNextStop() *StopBoundary {
	s.RLock()
	defer s.RUnlock()

	if s.stopBoundary == nil {
		return nil
	}

	// copy the value so we don't accidentally change it
	b := *s.stopBoundary
	return &b
}

// BlockProcessable should be called when new block is processable.
// It returns boolean indicating if the block should be processed.
func (s *StopControl) BlockProcessable(b *flow.Header) bool {
	s.Lock()
	defer s.Unlock()

	if s.state == StopControlOff {
		return true
	}

	if s.state == StopControlStopped {
		return false
	}

	// skips blocks at or above requested stopHeight
	if s.stopBoundary != nil && b.Height >= s.stopBoundary.StopHeight {
		s.log.Warn().
			Msgf(
				"Skipping execution of %s at height %d"+
					" because stop has been requested %s",
				b.ID(),
				b.Height,
				s.stopBoundary,
			)

		s.setState(StopControlStopping)
		return false
	}

	return true
}

// BlockFinalized should be called when a block is marked as finalized
func (s *StopControl) BlockFinalized(
	ctx context.Context,
	execState state.ReadOnlyExecutionState,
	h *flow.Header,
) {
	s.Lock()
	defer s.Unlock()

	if s.stopBoundary != nil ||
		s.state == StopControlOff ||
		s.state == StopControlStopped {
		return
	}

	// TODO: Version Beacons integration:
	// get VB from db index
	// check current node version against VB boundaries to determine when the next
	// stopping height should be. Move stopping height.
	// If stopping height was set manually, only move it if the new height is earlier.
	// Requirements:
	// - inject current protocol version
	// - inject a way to query VB from db index
	// - add a field to know if stopping height was set manually or through VB

	// Once finalization reached stopHeight we can be sure no other fork will be valid at this height,
	// if this block's parent has been executed, we are safe to stop or shouldCrash.
	// This will happen during normal execution, where blocks are executed before they are finalized.
	// However, it is possible that EN block computation progress can fall behind. In this case,
	// we want to crash only after the execution reached the stopHeight.
	if h.Height != s.stopBoundary.StopHeight {
		return
	}

	executed, err := state.IsBlockExecuted(ctx, execState, h.ParentID)
	if err != nil {
		// any error here would indicate unexpected storage error, so we crash the node
		// TODO: what if the error is due to the node being stopped?
		// i.e. context cancelled?
		s.log.Fatal().
			Err(err).
			Str("block_id", h.ID().String()).
			Msg("failed to check if the block has been executed")
		return
	}

	if executed {
		s.stopExecution()
		return
	}

	s.stopAfterExecuting = h.ParentID
	s.log.Info().
		Msgf(
			"Node scheduled to stop executing"+
				" after executing block %s at height %d",
			s.stopAfterExecuting.String(),
			h.Height-1,
		)
}

// OnBlockExecuted should be called after a block has finished execution
func (s *StopControl) OnBlockExecuted(h *flow.Header) {
	s.Lock()
	defer s.Unlock()

	if s.stopBoundary != nil ||
		s.state == StopControlOff ||
		s.state == StopControlStopped {
		return
	}

	if s.stopAfterExecuting != h.ID() {
		return
	}

	// double check. Even if requested stopHeight has been changed multiple times,
	// as long as it matches this block we are safe to terminate
	if s.stopBoundary != nil && h.Height != s.stopBoundary.StopHeight-1 {
		s.log.Warn().
			Msgf(
				"Inconsistent stopping state. "+
					"Scheduled to stop after executing block ID %s and height %d, "+
					"but this block has a height %d. ",
				h.ID().String(),
				s.stopBoundary.StopHeight-1,
				h.Height,
			)
		return
	}

	s.stopExecution()
}

func (s *StopControl) stopExecution() {
	if s.stopBoundary != nil && s.stopBoundary.ShouldCrash {
		s.log.Fatal().Msgf(
			"Crashing as finalization reached requested "+
				"stop %s and the highest executed block is the previous one",
			s.stopBoundary,
		)
		return
	}

	s.setState(StopControlStopped)

	s.log.Warn().Msgf(
		"Pausing execution as finalization reached "+
			"the requested stop height %s",
		s.stopBoundary,
	)
}
