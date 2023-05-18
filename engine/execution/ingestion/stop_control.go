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
//
// StopControl can stop execution or crash the node at a specific block height. The stop
// height can be set manually or by the version beacon service event. This leads to some
// edge cases that are handled by the StopControl:
//
//  1. stop is already set manually and is set again manually.
//     This is considered as an attempt to move the stop height. The resulting stop
//     height is the new one.
//  2. stop is already set manually and is set by the version beacon.
//     The resulting stop height is the earlier one.
//  3. stop is already set by the version beacon and is set manually.
//     The resulting stop height is the earlier one.
//  4. stop is already set by the version beacon and is set by the version beacon.
//     This means version boundaries were edited. The resulting stop
//     height is the new one.
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

	log := sc.log.With().
		Bool("node_will_react_to_version_beacon",
			sc.nodeVersion != nil).
		Logger()

	if sc.nodeVersion != nil {
		log = log.With().
			Stringer("node_version", sc.nodeVersion).
			Bool("crash_on_version_boundary_reached",
				sc.crashOnVersionBoundaryReached).
			Logger()
	}

	log.Info().Msgf("Created")

	// TODO: handle version beacon already indicating a stop
	// right now the stop will happen on first BlockFinalized
	// which is fine, but ideally we would stop right away

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

	log := s.log.With().
		Stringer("old_stop", s.stopBoundary).
		Stringer("new_stop", stopBoundary).
		Logger()

	canChange, reason := s.canChangeStop(
		stopBoundary.StopHeight,
		false,
	)
	if !canChange {
		err := fmt.Errorf(reason)

		log.Warn().Err(err).Msg("cannot set stopHeight")
		return err
	}

	log.Info().Msg("stop set")
	s.stopBoundary = stopBoundary

	return nil
}

// canChangeStop verifies if the stop parameters can be changed
// returns false and the reason if the parameters cannot be changed
// if newHeight == math.MaxUint64 tha basically means that the stop is being removed
func (s *StopControl) canChangeStop(
	newHeight uint64,
	fromVersionBeacon bool,
) (
	bool,
	string,
) {

	if s.stopped {
		return false, "cannot update stop parameters, already stopped"
	}

	if s.stopBoundary == nil {
		// if there is no stop boundary set, we can set it to anything
		return true, ""
	}

	if s.stopBoundary.cannotBeChanged {
		return false, fmt.Sprintf(
			"cannot update stopHeight, stopping commenced for %s",
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
		return false, "cannot update stopHeight, new stop height is later than the " +
			"current one (or removing a stop)"
	}

	return true, ""
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

	// handling errors here is a bit tricky because we cannot propagate the error out
	// TODO: handle this error better or use the same stopping mechanism as for the
	// stopHeight
	handleErr := func(err error) {
		s.log.Fatal().
			Err(err).
			Stringer("block_id", h.ID()).
			Stringer("stop", s.stopBoundary).
			Msg("un-handlabe error in stop control BlockFinalized")

		// s.stopExecution()
	}

	err := s.handleVersionBeacon(h.Height)
	if err != nil {
		handleErr(fmt.Errorf("failed to process version beacons: %w", err))
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
		s.log.Warn().
			Uint64("finalization_height", h.Height).
			Stringer("block_id", h.ID()).
			Stringer("stop", s.stopBoundary).
			Msg("Block finalization already beyond stop.")

		// Let's find the ID of the block that should be executed last
		// which is the parent of the block at the stopHeight
		header, err := s.headers.ByHeight(s.stopBoundary.StopHeight - 1)
		if err != nil {
			handleErr(fmt.Errorf("failed to get header by height: %w", err))
			return
		}
		parentID = header.ID()
	}

	s.stopBoundary.stopAfterExecuting = parentID

	s.log.Info().
		Stringer("block_id", h.ID()).
		Stringer("stop", s.stopBoundary).
		Stringer("stop_after_executing", s.stopBoundary.stopAfterExecuting).
		Msgf("Found ID of the block that should be executed last")

	// check if the parent block has been executed then stop right away
	executed, err := state.IsBlockExecuted(ctx, execState, h.ParentID)
	if err != nil {
		handleErr(fmt.Errorf(
			"failed to check if the block has been executed: %w",
			err,
		))
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

	if vb == nil {
		// no version beacon found
		// this is unexpected as there should always be at least the
		// starting version beacon, but not fatal.
		// It can happen if the node starts before bootstrap is finished.
		s.log.Info().
			Uint64("height", height).
			Msg("No version beacon found for stop control")
		return nil
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

	if !set && !shouldBeSet {
		// all good, no stop boundary set
		return nil
	}

	log := s.log.With().
		Stringer("old_stop", s.stopBoundary).
		Logger()

	canChange, reason := s.canChangeStop(
		math.MaxUint64,
		true,
	)
	if !canChange {
		log.Warn().
			Str("reason", reason).
			Msg("Cannot change stop boundary when detecting new version beacon")
		return nil
	}

	log.Info().Msg("stop cleared")

	var newStop *stopBoundary
	if shouldBeSet {
		newStop = &stopBoundary{
			StopParameters: StopParameters{
				StopHeight:  stopHeight,
				ShouldCrash: s.crashOnVersionBoundaryReached,
			},
			fromVersionBeacon: true,
		}
	}

	s.log.Info().
		Stringer("old_stop", s.stopBoundary).
		Stringer("new_stop", newStop).
		Msg("New stop set")

	s.stopBoundary = newStop

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
