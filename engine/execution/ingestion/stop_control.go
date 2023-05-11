package ingestion

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// StopControl is a specialized component used by ingestion.Engine to encapsulate
// control of stopping blocks execution.
// It is intended to work tightly with the Engine, not as a general mechanism or interface.
// StopControl follows states described in StopState
type StopControl struct {
	sync.RWMutex
	log zerolog.Logger

	stopped      bool
	stopBoundary *stopBoundary

	headers StopControlHeaders

	nodeVersion    *semver.Version
	versionBeacons storage.VersionBeacons
	versionBeacon  *flow.SealedVersionBeacon

	crashOnVersionBoundaryReached bool
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

	// if the stop parameters were set by the version beacon
	fromVersionBeacon bool
}

// String returns string in the format "crash@20023[versionBeacon]" or
// "crash@20023@blockID[versionBeacon]"
// block ID is only present if stopAfterExecuting is set
// the ID is from the block that should be executed last and has height one
// less than StopHeight
func (s *stopBoundary) String() string {
	if s == nil {
		return "none"
	}

	sb := strings.Builder{}
	if s.ShouldCrash {
		sb.WriteString("crash")
	} else {
		sb.WriteString("stop")
	}
	sb.WriteString("@")
	sb.WriteString(fmt.Sprintf("%d", s.StopHeight))

	if s.stopAfterExecuting != flow.ZeroID {
		sb.WriteString("@")
		sb.WriteString(s.stopAfterExecuting.String())
	}
	if s.fromVersionBeacon {
		sb.WriteString("[versionBeacon]")
	} else {
		sb.WriteString("[manual]")
	}

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

func StopControlWithVersionControl(
	nodeVersion *semver.Version,
	versionBeacons storage.VersionBeacons,
	crashOnVersionBoundaryReached bool,
) StopControlOption {
	return func(s *StopControl) {
		s.nodeVersion = nodeVersion
		s.versionBeacons = versionBeacons
		s.crashOnVersionBoundaryReached = crashOnVersionBoundaryReached
	}
}

// StopControlHeaders is an interface for fetching headers
// Its jut a small subset of storage.Headers for comments see storage.Headers
type StopControlHeaders interface {
	ByHeight(height uint64) (*flow.Header, error)
}

// NewStopControl creates new empty NewStopControl
func NewStopControl(
	headers StopControlHeaders,
	options ...StopControlOption,
) *StopControl {

	sc := &StopControl{
		log:     zerolog.Nop(),
		headers: headers,
	}

	for _, option := range options {
		option(sc)
	}

	log := sc.log

	if sc.nodeVersion != nil {
		log = log.With().
			Stringer("node_version", sc.nodeVersion).
			Bool("crash_on_version_boundary_reached", sc.crashOnVersionBoundaryReached).
			Logger()
	}

	log.Info().Msgf("Created")

	// TODO: handle version beacon already indicating a stop

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

	stopBoundary := &stopBoundary{
		StopParameters: stop,
	}

	return s.unsafeSetStop(stopBoundary)
}

// unsafeSetStop is the same as SetStop but without locking, so it can be
// called internally
func (s *StopControl) unsafeSetStop(
	boundary *stopBoundary,
) error {
	log := s.log.With().
		Stringer("old_stop", s.stopBoundary).
		Stringer("new_stop", boundary).
		Logger()

	err := s.verifyCanChangeStop(
		boundary.StopHeight,
		boundary.fromVersionBeacon,
	)
	if err != nil {
		log.Warn().Err(err).Msg("cannot set stopHeight")
		return err
	}

	log.Info().Msg("stop set")
	s.stopBoundary = boundary

	return nil
}

// unsafeUnsetStop clears the stop
// there is no locking
// this is needed, because version beacons can change remove future stops
func (s *StopControl) unsafeUnsetStop() error {
	log := s.log.With().
		Stringer("old_stop", s.stopBoundary).
		Logger()

	err := s.verifyCanChangeStop(
		math.MaxUint64,
		true,
	)
	if err != nil {
		log.Warn().Err(err).Msg("cannot clear stopHeight")
		return err
	}

	log.Info().Msg("stop cleared")
	s.stopBoundary = nil

	return nil
}

// verifyCanChangeStop verifies if the stop parameters can be changed
// returns error if the parameters cannot be changed
// if newHeight == math.MaxUint64 tha basically means that the stop is being removed
func (s *StopControl) verifyCanChangeStop(
	newHeight uint64,
	fromVersionBeacon bool,
) error {

	if s.stopped {
		return fmt.Errorf("cannot update stop parameters, already stopped")
	}

	if s.stopBoundary == nil {
		// if there is no stop boundary set, we can set it to anything
		return nil
	}

	if s.stopBoundary.cannotBeChanged {
		return fmt.Errorf(
			"cannot update stopHeight, "+
				"stopping commenced for %s",
			s.stopBoundary,
		)
	}

	if s.stopBoundary.fromVersionBeacon != fromVersionBeacon &&
		newHeight > s.stopBoundary.StopHeight {
		// if one stop was set by the version beacon and the other one was manual
		// we can only update if the new stop is earlier

		// this prevents users moving the stopHeight forward when a version boundary
		// is earlier, and prevents version beacons from moving the stopHeight forward
		// when a manual stop is earlier.
		return fmt.Errorf("cannot update stopHeight, " +
			"new stop height is later than the current one (or removing a stop)")
	}

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
//
// Once finalization reached stopHeight we can be sure no other fork will be valid at
// this height, if this block's parent has been executed, we are safe to stop.
// This will happen during normal execution, where blocks are executed
// before they are finalized. However, it is possible that EN block computation
// progress can fall behind. In this case, we want to crash only after the execution
// reached the stopHeight.
func (s *StopControl) BlockFinalized(
	ctx context.Context,
	execState state.ReadOnlyExecutionState,
	h *flow.Header,
) {
	s.Lock()
	defer s.Unlock()

	// we already know the ID of the block that should be executed last nothing to do
	// node is stopping
	if s.stopBoundary != nil &&
		s.stopBoundary.stopAfterExecuting != flow.ZeroID {
		return
	}

	// already stopped, nothing to do
	if s.stopped {
		return
	}

	log := s.log.With().
		Stringer("block_id", h.ID()).
		Stringer("stop", s.stopBoundary).
		Logger()

	err := s.handleVersionBeacon(h.Height)
	if err != nil {
		// TODO: handle this error better
		log.Fatal().
			Err(err).
			Msg("failed to process version beacons")
		return
	}

	// no stop is set, nothing to do
	if s.stopBoundary == nil {
		return
	}

	// we are not at the stop yet, nothing to do
	if h.Height < s.stopBoundary.StopHeight {
		return
	}

	parentID := h.ParentID

	if h.Height != s.stopBoundary.StopHeight {
		// we are past the stop. This can happen if stop was set before
		// last finalized block
		log.Warn().
			Uint64("finalization_height", h.Height).
			Msg("Block finalization already beyond stop.")

		// Let's find the ID of the block that should be executed last
		// which is the parent of the block at the stopHeight
		header, err := s.headers.ByHeight(s.stopBoundary.StopHeight - 1)
		if err != nil {
			// TODO: handle this error better
			log.Fatal().
				Err(err).
				Msg("failed to get header by height")
			return
		}
		parentID = header.ID()
	}

	s.stopBoundary.stopAfterExecuting = parentID

	log.Info().
		Stringer("stop_after_executing", s.stopBoundary.stopAfterExecuting).
		Msgf("Found ID of the block that should be executed last")

	// check if the parent block has been executed then stop right away
	executed, err := state.IsBlockExecuted(ctx, execState, h.ParentID)
	if err != nil {
		// any error here would indicate unexpected storage error, so we crash the node
		// TODO: what if the error is due to the node being stopped?
		// i.e. context cancelled?
		// do this more gracefully
		log.Fatal().
			Err(err).
			Msg("failed to check if the block has been executed")
		return
	}

	if executed {
		// we already reached the point where we should stop
		s.stopExecution()
		return
	}
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
	if h.Height != s.stopBoundary.StopHeight-1 {
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

	s.stopped = true
	log.Warn().Msg("Stopping as finalization reached requested stop")

	if s.stopBoundary != nil && s.stopBoundary.ShouldCrash {
		// TODO: crash more gracefully or at least in a more explicit way
		log.Fatal().Msg("Crashing as finalization reached requested stop")
		return
	}
}

func (s *StopControl) handleVersionBeacon(
	height uint64,
) error {
	if s.nodeVersion == nil || s.stopped {
		return nil
	}

	if s.versionBeacon != nil && s.versionBeacon.SealHeight >= height {
		// we already processed this or a higher version beacon
		return nil
	}

	vb, err := s.versionBeacons.Highest(height)
	if err != nil {
		return fmt.Errorf("failed to get highest "+
			"version beacon for stop control: %w", err)
	}

	if s.versionBeacon != nil && s.versionBeacon.SealHeight >= vb.SealHeight {
		// we already processed this or a higher version beacon
		return nil
	}

	s.log.Info().
		Uint64("vb_seal_height", vb.SealHeight).
		Uint64("vb_sequence", vb.Sequence).
		Msg("New version beacon found")

	// this is now the last handled version beacon
	s.versionBeacon = vb

	// this is a new version beacon check what boundary it sets
	stopHeight, shouldBeSet, err := s.getVersionBeaconStopHeight(vb)
	if err != nil {
		return err
	}

	set := s.stopBoundary != nil

	if shouldBeSet {
		// we need to set the new boundary whether it was set before or not
		// in case a stop has been set let the SetStop decide which is more important
		err := s.unsafeSetStop(&stopBoundary{
			StopParameters: StopParameters{
				StopHeight:  stopHeight,
				ShouldCrash: s.crashOnVersionBoundaryReached,
			},
			fromVersionBeacon: true,
		})
		if err != nil {
			// Invalid stop condition. Either already stopped or stopping
			//or a stop is scheduled earlier.
			// This is ok, we just log it and ignore it
			// TODO: clean this up, we should not use errors for this kind of control flow
			s.log.Info().
				Err(err).
				Msg("Failed to set stop boundary from version beacon. ")

		}
		return nil
	}

	if !set {
		// all good, no stop boundary set
		return nil
	}

	// we need to remove the stop boundary,
	// but only if it was set by a version beacon
	err = s.unsafeUnsetStop()
	if err != nil {
		// Invalid stop condition. Either already stopped or stopping
		//or a stop is scheduled earlier.
		// This is ok, we just log it and ignore it
		// TODO: clean this up, we should not use errors for this kind of control flow
		s.log.Info().
			Err(err).
			Msg("Failed to set stop boundary from version beacon. ")
	}
	return nil
}

// getVersionBeaconStopHeight returns the stop height that should be set
// based on the version beacon
// error is not expected during normal operation since the version beacon
// should have been validated when indexing
func (s *StopControl) getVersionBeaconStopHeight(
	vb *flow.SealedVersionBeacon,
) (
	uint64,
	bool,
	error,
) {
	// version boundaries are sorted by version
	for _, boundary := range vb.VersionBoundaries {
		ver, err := boundary.Semver()
		if err != nil || ver == nil {
			// this should never happen as we already validated the version beacon
			// when indexing it
			return 0, false, fmt.Errorf("failed to parse semver: %w", err)
		}

		if s.nodeVersion.LessThan(*ver) {
			// we need to stop here
			return boundary.BlockHeight, true, nil
		}
	}
	return 0, false, nil
}
