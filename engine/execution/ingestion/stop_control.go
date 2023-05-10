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
// control of stopping blocks execution.
// It is intended to work tightly with the Engine, not as a general mechanism or interface.
// StopControl follows states described in StopState
type StopControl struct {
	sync.RWMutex
	log zerolog.Logger

	stopBoundary *stopBoundary

	stopped bool
}

type StopParameters struct {
	// desired StopHeight, the first value new version should be used,
	// so this height WON'T be executed
	StopHeight uint64

	// if the node should crash or just pause after reaching StopHeight
	ShouldCrash bool
}

type stopBoundary struct {
	StopParameters

	// once the StopParameters are reached they cannot be changed
	cannotBeChanged bool

	// This is the block ID of the block that should be executed last.
	stopAfterExecuting flow.Identifier
}

// String returns string in the format "crash@20023"
func (s *stopBoundary) String() string {
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

type StopControlOption func(*StopControl)

func StopControlWithLogger(log zerolog.Logger) StopControlOption {
	return func(s *StopControl) {
		s.log = log.With().Str("component", "stop_control").Logger()
	}
}

func StopControlWithStopped() StopControlOption {
	return func(s *StopControl) {
		s.stopped = true
	}
}

// NewStopControl creates new empty NewStopControl
func NewStopControl(
	options ...StopControlOption,
) *StopControl {

	sc := &StopControl{
		log: zerolog.Nop(),
	}

	for _, option := range options {
		option(sc)
	}

	sc.log.Debug().Msgf("Created")

	return sc
}

// IsExecutionStopped returns true is block execution has been stopped
func (s *StopControl) IsExecutionStopped() bool {
	s.RLock()
	defer s.RUnlock()

	return s.stopped
}

// SetStop sets new stop parameters.
// Returns error if the stopping process has already commenced, or if already stopped.
func (s *StopControl) SetStop(
	stop StopParameters,
) error {
	s.Lock()
	defer s.Unlock()

	if s.stopped {
		return fmt.Errorf("cannot update stop parameters, already stopped")
	}

	if s.stopBoundary != nil && s.stopBoundary.cannotBeChanged {
		return fmt.Errorf(
			"cannot update stopHeight, "+
				"stopping commenced for %s",
			s.stopBoundary,
		)
	}

	stopBoundary := &stopBoundary{
		StopParameters: stop,
	}

	s.log.Info().
		Stringer("old_stop", s.stopBoundary).
		Stringer("new_stop", stopBoundary).
		Msg("new stopHeight set")

	s.stopBoundary = stopBoundary

	return nil
}

// GetStop returns the upcoming stop parameters or nil if no stop is set.
func (s *StopControl) GetStop() *StopParameters {
	s.RLock()
	defer s.RUnlock()

	if s.stopBoundary == nil {
		return nil
	}

	p := s.stopBoundary.StopParameters
	return &p
}

// BlockProcessable should be called when new block is processable.
// It returns boolean indicating if the block should be processed.
func (s *StopControl) BlockProcessable(b *flow.Header) bool {
	s.Lock()
	defer s.Unlock()

	// don't process anymore blocks if stopped
	if s.stopped {
		return false
	}

	// if no stop is set process all blocks
	if s.stopBoundary == nil {
		return true
	}

	// Skips blocks at or above requested stopHeight
	// doing so means we have started the stopping process
	if b.Height >= s.stopBoundary.StopHeight {
		s.log.Warn().
			Msgf(
				"Skipping execution of %s at height %d"+
					" because stop has been requested %s",
				b.ID(),
				b.Height,
				s.stopBoundary,
			)

		s.stopBoundary.cannotBeChanged = true
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

	if s.stopBoundary == nil || s.stopped {
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
		// do this more gracefully
		s.log.Fatal().
			Err(err).
			Str("block_id", h.ID().String()).
			Msg("failed to check if the block has been executed")
		return
	}

	if executed {
		// we already reached the point where we should stop
		s.stopExecution()
		return
	}

	s.stopBoundary.stopAfterExecuting = h.ParentID
	s.log.Info().
		Msgf(
			"Node scheduled to stop executing"+
				" after executing block %s at height %d",
			s.stopBoundary.stopAfterExecuting.String(),
			h.Height-1,
		)
}

// OnBlockExecuted should be called after a block has finished execution
func (s *StopControl) OnBlockExecuted(h *flow.Header) {
	s.Lock()
	defer s.Unlock()

	if s.stopBoundary == nil || s.stopped {
		return
	}

	if s.stopBoundary.stopAfterExecuting != h.ID() {
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
	log := s.log.With().
		Stringer("requested_stop", s.stopBoundary).
		Uint64("last_executed_height", s.stopBoundary.StopHeight).
		Stringer("last_executed_id", s.stopBoundary.stopAfterExecuting).
		Logger()

	if s.stopBoundary != nil && s.stopBoundary.ShouldCrash {
		// TODO: crash more gracefully or at least in a more explicit way
		log.Fatal().Msg("Crashing as finalization reached requested stop")
		return
	}

	s.stopped = true
	log.Warn().Msg("Stopping as finalization reached requested stop")
}
