package stop

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// StopControl is a specialized component used by ingestion.Engine to encapsulate
// control of stopping blocks execution.
//
// StopControl can stop execution or crash the node before a specific block height. The
// stop height can be set manually or by the version beacon service event. This leads to
// some edge cases that are handled by the StopControl:
//
//  1. stop is already set manually and is set again manually.
//     This is considered as an attempt to move the stop height. The resulting stop
//     height is the new one. Note, the new height can be either lower or higher than
//     previous value.
//  2. stop is already set manually and is set by the version beacon.
//     The resulting stop height is the lower one.
//  3. stop is already set by the version beacon and is set manually.
//     The resulting stop height is the lower one.
//  4. stop is already set by the version beacon and is set by the version beacon.
//     This means version boundaries were edited. The resulting stop
//     height is the new one.
//
// The stop control uses the finalized and executed distributor to get the latest
// finalized and executed block. The distributor is used to avoid the need to subscribe
// and handle the finalized and executed events.'
type StopControl struct {
	sync.RWMutex
	component.Component
	cm *component.ComponentManager

	blockFinalizedAndExecutedChan chan *flow.Header

	versionBeacons storage.VersionBeacons

	// stopped is true if node should no longer be executing blocs.
	stopped bool
	// stopBoundary is when the node should stop.
	stopBoundary stopBoundary
	// nodeVersion could be nil right now. See NewStopControl.
	nodeVersion *semver.Version
	// last seen version beacon, used to detect version beacon changes
	versionBeacon *flow.SealedVersionBeacon
	// if the node should crash on version boundary from a version beacon is reached
	crashOnVersionBoundaryReached bool

	log zerolog.Logger
}

var NoStopHeight = uint64(math.MaxUint64)

type StopParameters struct {
	// desired StopBeforeHeight, the first value new version should be used,
	// so this height WON'T be executed
	StopBeforeHeight uint64

	// if the node should crash or just pause after reaching StopBeforeHeight
	ShouldCrash bool
}

func (p StopParameters) Set() bool {
	return p.StopBeforeHeight != NoStopHeight
}

type stopBoundary struct {
	StopParameters

	// once the StopParameters are reached they cannot be changed
	immutable bool

	// if the stop parameters were set by the version beacon
	fromVersionBeacon bool
}

// String returns string in the format "crash@20023[versionBeacon]" or
// "stop@20023[manual]"
// block ID is only present if stopAfterExecuting is set
// the ID is from the block that should be executed last and has height one
// less than StopBeforeHeight
func (s stopBoundary) String() string {
	if !s.Set() {
		return "none"
	}

	sb := strings.Builder{}
	if s.ShouldCrash {
		sb.WriteString("crash")
	} else {
		sb.WriteString("stop")
	}
	sb.WriteString("@")
	sb.WriteString(fmt.Sprintf("%d", s.StopBeforeHeight))

	if s.fromVersionBeacon {
		sb.WriteString("[versionBeacon]")
	} else {
		sb.WriteString("[manual]")
	}

	return sb.String()
}

// NewStopControl creates new StopControl.
//
// We currently have no strong guarantee that the node version is a valid semver.
// See build.SemverV2 for more details. That is why nil is a valid input for node version
// without a node version, the stop control can still be used for manual stopping.
func NewStopControl(
	log zerolog.Logger,
	versionBeacons storage.VersionBeacons,
	nodeVersion *semver.Version,
	latestFinalizedBlock *flow.Header,
	withStoppedExecution bool,
	crashOnVersionBoundaryReached bool,
) *StopControl {
	// We should not miss block finalized or executed events,
	// and we should be able to handle them faster than they are produced anyway.
	blockFinalizedChan := make(chan *flow.Header, 100)

	sc := &StopControl{
		log: log.With().
			Str("component", "stop_control").
			Logger(),

		blockFinalizedAndExecutedChan: blockFinalizedChan,

		nodeVersion:                   nodeVersion,
		versionBeacons:                versionBeacons,
		stopped:                       withStoppedExecution,
		crashOnVersionBoundaryReached: crashOnVersionBoundaryReached,
		// the default is to never stop
		stopBoundary: stopBoundary{
			StopParameters: StopParameters{
				StopBeforeHeight: NoStopHeight,
			},
		},
	}

	if sc.nodeVersion != nil {
		log = log.With().
			Stringer("node_version", sc.nodeVersion).
			Bool("crash_on_version_boundary_reached",
				sc.crashOnVersionBoundaryReached).
			Logger()
	}

	log.Info().Msgf("Created")

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(sc.processEvents)
	cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		sc.checkInitialVersionBeacon(ctx, ready, latestFinalizedBlock)
	})

	sc.cm = cm.Build()
	sc.Component = sc.cm

	return sc
}

// FinalizedAndExecuted is called when a block is finalized and executed.
func (s *StopControl) FinalizedAndExecuted(h *flow.Header) {
	s.blockFinalizedAndExecutedChan <- h
}

// processEvents is a worker that processes block finalized and executed events.
func (s *StopControl) processEvents(
	ctx irrecoverable.SignalerContext,
	ready component.ReadyFunc,
) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case h := <-s.blockFinalizedAndExecutedChan:
			s.blockFinalizedAndExecuted(ctx, h)
		}
	}
}

// BlockFinalizedAndExecutedForTesting is used for testing	only.
func (s *StopControl) BlockFinalizedAndExecutedForTesting(h *flow.Header) {
	s.blockFinalizedAndExecuted(irrecoverable.MockSignalerContext{}, h)
}

// checkInitialVersionBeacon is a worker that checks the initial version beacon.
// It does not signal ready until the initial version beacon is checked.
func (s *StopControl) checkInitialVersionBeacon(
	ctx irrecoverable.SignalerContext,
	ready component.ReadyFunc,
	latestFinalizedBlock *flow.Header,
) {
	// component is not ready until we checked the initial version beacon
	defer ready()

	// the most straightforward way to check it is to simply pretend we just finalized and
	// executed the last finalized and executed block
	s.blockFinalizedAndExecuted(ctx, latestFinalizedBlock)
}

// IsExecutionStopped returns true is block execution has been stopped
func (s *StopControl) IsExecutionStopped() bool {
	s.RLock()
	defer s.RUnlock()

	return s.stopped
}

// SetStopParameters sets new stop parameters manually.
//
// Expected error returns during normal operations:
//   - ErrCannotChangeStop: this indicates that new stop parameters cannot be set.
//     See stop.validateStopChange.
func (s *StopControl) SetStopParameters(
	stop StopParameters,
) error {
	s.Lock()
	defer s.Unlock()

	boundary := stopBoundary{
		StopParameters:    stop,
		fromVersionBeacon: false,
	}

	return s.setStopParameters(boundary)
}

// setStopParameters sets new stop parameters.
// stopBoundary is the new stop parameters. If nil, the stop is removed.
//
// Expected error returns during normal operations:
//   - ErrCannotChangeStop: this indicates that new stop parameters cannot be set.
//     See stop.validateStopChange.
//
// Caller must acquire the lock.
func (s *StopControl) setStopParameters(
	stopBoundary stopBoundary,
) error {
	log := s.log.With().
		Stringer("old_stop", s.stopBoundary).
		Stringer("new_stop", stopBoundary).
		Logger()

	err := s.validateStopChange(stopBoundary)
	if err != nil {
		log.Info().Err(err).Msg("cannot set stopHeight")
		return err
	}

	log.Info().Msg("new stop set")
	s.stopBoundary = stopBoundary

	return nil
}

var ErrCannotChangeStop = errors.New("cannot change stop control stopping parameters")

// validateStopChange verifies if the stop parameters can be changed
// returns the error with the reason if the parameters cannot be changed.
//
// Stop parameters cannot be changed if:
//   - node is already stopped
//   - stop parameters are immutable (due to them already affecting execution see
//     ShouldExecuteBlock)
//   - stop parameters are already set by a different source and the new stop is later than
//     the existing one
//
// Expected error returns during normal operations:
//   - ErrCannotChangeStop: this indicates that new stop parameters cannot be set.
//
// Caller must acquire the lock.
func (s *StopControl) validateStopChange(
	newStopBoundary stopBoundary,
) error {

	errf := func(reason string) error {
		return fmt.Errorf("%s: %w", reason, ErrCannotChangeStop)
	}

	if s.stopped {
		return errf("cannot update stop parameters, already stopped")
	}

	if s.stopBoundary.immutable {
		return errf(
			fmt.Sprintf(
				"cannot update stopHeight, stopping commenced for %s",
				s.stopBoundary),
		)
	}

	if !s.stopBoundary.Set() {
		// if the current stop is no stop, we can always update
		return nil
	}

	if s.stopBoundary.fromVersionBeacon == newStopBoundary.fromVersionBeacon {
		// if the stop was set by the same source, we can always update
		return nil
	}

	// if one stop was set by the version beacon and the other one was manual
	// we can only update if the new stop is strictly earlier
	if newStopBoundary.StopBeforeHeight < s.stopBoundary.StopBeforeHeight {
		return nil

	}
	// this prevents users moving the stopHeight forward when a version newStopBoundary
	// is earlier, and prevents version beacons from moving the stopHeight forward
	// when a manual stop is earlier.
	return errf("cannot update stopHeight, " +
		"new stop height is later than the current one")
}

// GetStopParameters returns the upcoming stop parameters or nil if no stop is set.
func (s *StopControl) GetStopParameters() StopParameters {
	s.RLock()
	defer s.RUnlock()

	return s.stopBoundary.StopParameters
}

// ShouldExecuteBlock should be called when new block can be executed.
// The block should not be executed if its height is above or equal to
// s.stopBoundary.StopBeforeHeight.
//
// It returns a boolean indicating if the block should be executed.
func (s *StopControl) ShouldExecuteBlock(b *flow.Header) bool {
	s.Lock()
	defer s.Unlock()

	// don't process anymore blocks if stopped
	if s.stopped {
		return false
	}

	// Skips blocks at or above requested stopHeight
	// doing so means we have started the stopping process
	if b.Height < s.stopBoundary.StopBeforeHeight {
		return true
	}

	s.log.Info().
		Msgf("Skipping execution of %s at height %d"+
			" because stop has been requested %s",
			b.ID(),
			b.Height,
			s.stopBoundary)

	// stopBoundary is now immutable, because it started affecting execution
	s.stopBoundary.immutable = true
	return false
}

// blockFinalizedAndExecuted is called when a block is marked as finalized and executed.
//
// Once finalization reached stopHeight we can be sure no other fork will be valid at
// this height, if this block's parent has been executed, we are safe to stop.
// This will happen during normal execution, where blocks are executed
// before they are finalized. However, it is possible that EN block computation
// progress can fall behind. In this case, we want to crash only after the execution
// reached the stopHeight.
func (s *StopControl) blockFinalizedAndExecuted(
	ctx irrecoverable.SignalerContext,
	h *flow.Header,
) {
	s.Lock()
	defer s.Unlock()

	// already stopped, nothing to do
	if s.stopped {
		return
	}

	s.processNewVersionBeacons(ctx, h.Height)

	// we are not at the stop yet, nothing to do
	if h.Height+1 < s.stopBoundary.StopBeforeHeight {
		return
	}

	if h.Height+1 != s.stopBoundary.StopBeforeHeight {
		// we are past the stop. This can happen if stop was set for a past height
		// this is unusual, but better to sto then to ignore the stop.
		s.log.Warn().
			Uint64("finalization_and_execution_height", h.Height).
			Stringer("block_id", h.ID()).
			Stringer("stop", s.stopBoundary).
			Msg("Block finalization and execution already beyond stop.")
	}

	s.stopExecution()
}

// stopExecution stops the node execution and crashes the node if ShouldCrash is true.
// Caller must acquire the lock.
func (s *StopControl) stopExecution() {
	log := s.log.With().
		Stringer("requested_stop", s.stopBoundary).
		Uint64("last_finalized_and_executed_height", s.stopBoundary.StopBeforeHeight-1).
		Logger()

	s.stopped = true
	log.Warn().Msg("Stopping as finalization and execution reached requested stop")

	if s.stopBoundary.ShouldCrash {
		// TODO: crash more gracefully or at least in a more explicit way
		log.Fatal().Msg("Crashing as finalization and execution reached requested stop")
		return
	}
}

// processNewVersionBeacons processes version beacons and updates the stop control stop
// height if needed.
//
// When a block is finalized it is possible that a new version beacon is indexed.
// This new version beacon might have added/removed/moved a version boundary.
// The old version beacon is considered invalid, and the stop height must be updated
// according to the new version beacon.
//
// Caller must acquire the lock.
func (s *StopControl) processNewVersionBeacons(
	ctx irrecoverable.SignalerContext,
	height uint64,
) {
	// TODO: remove when we can guarantee that the node will always have a valid version
	if s.nodeVersion == nil {
		return
	}

	if s.versionBeacon != nil && s.versionBeacon.SealHeight >= height {
		// already processed this or a higher version beacon
		return
	}

	vb, err := s.versionBeacons.Highest(height)
	if err != nil {
		s.log.Err(err).
			Uint64("height", height).
			Msg("Failed to get highest version beacon for stop control")

		ctx.Throw(
			fmt.Errorf(
				"failed to get highest version beacon for stop control: %w",
				err))
		return
	}

	if vb == nil {
		// no version beacon found
		// this is unexpected as there should always be at least the
		// starting version beacon, but not fatal.
		// It can happen if the node starts before bootstrap is finished.
		// TODO: remove when we can guarantee that there will always be a version beacon
		s.log.Info().
			Uint64("height", height).
			Msg("No version beacon found for stop control")
		return
	}

	if s.versionBeacon != nil && s.versionBeacon.SealHeight >= vb.SealHeight {
		// we already processed this or a higher version beacon
		return
	}

	s.log.Info().
		Uint64("vb_seal_height", vb.SealHeight).
		Uint64("vb_sequence", vb.Sequence).
		Msg("New version beacon found")

	// this is now the last handled version beacon
	s.versionBeacon = vb

	// this is a new version beacon check what boundary it sets
	stopHeight, err := s.getVersionBeaconStopHeight(vb)
	if err != nil {
		s.log.Err(err).
			Interface("version_beacon", vb).
			Msg("Failed to get stop height from version beacon")

		ctx.Throw(
			fmt.Errorf("failed to get stop height from version beacon: %w", err))
		return
	}

	var newStop = stopBoundary{
		StopParameters: StopParameters{
			StopBeforeHeight: stopHeight,
			ShouldCrash:      s.crashOnVersionBoundaryReached,
		},
		fromVersionBeacon: true,
	}

	err = s.setStopParameters(newStop)
	if err != nil {
		// This is just informational and is expected to sometimes happen during
		// normal operation. The causes for this are described here: validateStopChange.
		s.log.Info().
			Err(err).
			Msg("Cannot change stop boundary when detecting new version beacon")
	}
}

// getVersionBeaconStopHeight returns the stop height that should be set
// based on the version beacon
//
// No error is expected during normal operation since the version beacon
// should have been validated when indexing.
//
// Caller must acquire the lock.
func (s *StopControl) getVersionBeaconStopHeight(
	vb *flow.SealedVersionBeacon,
) (
	uint64,
	error,
) {
	// version boundaries are sorted by version
	for _, boundary := range vb.VersionBoundaries {
		ver, err := boundary.Semver()
		if err != nil || ver == nil {
			// this should never happen as we already validated the version beacon
			// when indexing it
			return 0, fmt.Errorf("failed to parse semver: %w", err)
		}

		// This condition can be tweaked in the future. For example if we guarantee that
		// all nodes with the same major version have compatible execution,
		// we can stop only on major version change.
		if s.nodeVersion.LessThan(*ver) {
			// we need to stop here
			return boundary.BlockHeight, nil
		}
	}

	// no stop boundary should be set
	return NoStopHeight, nil
}
