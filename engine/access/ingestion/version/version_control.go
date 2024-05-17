package version

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	psEvents "github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
)

type VersionControlConsumer func(height uint64, semver string)

// VersionControl TODO
type VersionControl struct {
	unit                    *engine.Unit
	maxGracefulStopDuration time.Duration

	// Version control needs to consume BlockFinalized events.
	// adding psEvents.Noop makes it a protocol.Consumer
	psEvents.Noop
	sync.RWMutex
	component.Component

	blockFinalizedChan chan *flow.Header

	headers        storage.Headers
	versionBeacons storage.VersionBeacons

	// stopBoundary is when the node should stop.
	stopBoundary stopBoundary
	// nodeVersion could be nil right now. See NewVersionControl.
	nodeVersion *semver.Version
	// last seen version beacon, used to detect version beacon changes
	versionBeacon *flow.SealedVersionBeacon

	consumers []VersionControlConsumer

	log zerolog.Logger
}

var _ protocol.Consumer = (*VersionControl)(nil)
var _ component.Component = (*VersionControl)(nil)

var NoStopHeight = uint64(math.MaxUint64)

type stopBoundary struct {
	// desired StopBeforeHeight, the first value new version should be used,
	// so this height WON'T be executed
	StopBeforeHeight uint64

	// The stop control will prevent execution of blocks higher than StopBeforeHeight
	// once this happens the stop control is affecting execution and StopParameters can
	// no longer be changed
	immutable bool

	// This is the block ID of the block that should be executed last.
	stopAfterExecuting flow.Identifier
}

func (v *stopBoundary) Set() bool {
	return v.StopBeforeHeight != NoStopHeight
}

// String returns string in the format "crash@20023[stopBoundarySourceVersionBeacon]" or
// "stop@20023@blockID[manual]"
// block ID is only present if stopAfterExecuting is set
// the ID is from the block that should be executed last and has height one
// less than StopBeforeHeight
func (v *stopBoundary) String() string {
	if !v.Set() {
		return "none"
	}

	sb := strings.Builder{}
	sb.WriteString("crash")
	sb.WriteString("@")
	sb.WriteString(fmt.Sprintf("%d", v.StopBeforeHeight))

	if v.stopAfterExecuting != flow.ZeroID {
		sb.WriteString("@")
		sb.WriteString(v.stopAfterExecuting.String())
	}

	return sb.String()
}

// NewVersionControl creates new VersionControl.
//
// We currently have no strong guarantee that the node version is a valid semver.
// See build.SemverV2 for more details. That is why nil is a valid input for node version
// without a node version, the stop control can still be used for manual stopping.
func NewVersionControl(
	unit *engine.Unit,
	maxGracefulStopDuration time.Duration,
	log zerolog.Logger,
	headers storage.Headers,
	versionBeacons storage.VersionBeacons,
	nodeVersion *semver.Version,
	latestFinalizedBlock *flow.Header,
) *VersionControl {
	// We should not miss block finalized events, and we should be able to handle them
	// faster than they are produced anyway.
	blockFinalizedChan := make(chan *flow.Header, 1000)

	sc := &VersionControl{
		unit:                    unit,
		maxGracefulStopDuration: maxGracefulStopDuration,
		log: log.With().
			Str("component", "version_control").
			Logger(),

		blockFinalizedChan: blockFinalizedChan,

		headers:        headers,
		nodeVersion:    nodeVersion,
		versionBeacons: versionBeacons,
		stopBoundary: stopBoundary{
			StopBeforeHeight: NoStopHeight,
		},
	}

	if sc.nodeVersion != nil {
		log = log.With().
			Stringer("node_version", sc.nodeVersion).
			Logger()
	}

	log.Info().Msgf("Created")

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(sc.processEvents)
	cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		sc.checkInitialVersionBeacon(ctx, ready, latestFinalizedBlock)
	})

	sc.Component = cm.Build()

	// TODO: handle version beacon already indicating a stop
	// right now the stop will happen on first BlockFinalized
	// which is fine, but ideally we would stop right away.

	return sc
}

// BlockFinalized is called when a block is finalized.
//
// This is a protocol event consumer. See protocol.Consumer.
func (v *VersionControl) BlockFinalized(h *flow.Header) {
	v.blockFinalizedChan <- h
}

func (v *VersionControl) CompatibleAtBlock(height uint64) bool {
	//TODO: Implement it
	return false
}

func (v *VersionControl) AddVersionUpdatesConsumer(consumer func(height uint64, semver string)) {
	v.Lock()
	defer v.Unlock()

	v.consumers = append(v.consumers, consumer)
}

// processEvents is a worker that processes block finalized events.
func (v *VersionControl) processEvents(
	ctx irrecoverable.SignalerContext,
	ready component.ReadyFunc,
) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case h := <-v.blockFinalizedChan:
			v.blockFinalized(ctx, h)
		}
	}
}

// BlockFinalizedForTesting is used for testing	only.
func (v *VersionControl) BlockFinalizedForTesting(h *flow.Header) {
	v.blockFinalized(irrecoverable.MockSignalerContext{}, h)
}

func (v *VersionControl) checkInitialVersionBeacon(
	ctx irrecoverable.SignalerContext,
	ready component.ReadyFunc,
	latestFinalizedBlock *flow.Header,
) {
	// component is not ready until we checked the initial version beacon
	defer ready()

	// the most straightforward way to check it is to simply pretend we just finalized the
	// last finalized block
	v.blockFinalized(ctx, latestFinalizedBlock)

}

// SetStopBeforeHeight sets new stop parameters manually.
//
// Expected error returns during normal operations:
//   - ErrCannotChangeStop: this indicates that new stop parameters cannot be set.
//     See stop.validateStopChange.
func (v *VersionControl) SetStopBeforeHeight(
	stopBeforeHeight uint64,
) error {
	v.Lock()
	defer v.Unlock()

	boundary := stopBoundary{
		StopBeforeHeight: stopBeforeHeight,
	}

	return v.setStopParameters(boundary)
}

// setStopParameters sets new stop parameters.
// stopBoundary is the new stop parameters. If nil, the stop is removed.
//
// Expected error returns during normal operations:
//   - ErrCannotChangeStop: this indicates that new stop parameters cannot be set.
//     See stop.validateStopChange.
//
// Caller must acquire the lock.
func (v *VersionControl) setStopParameters(
	stopBoundary stopBoundary,
) error {
	log := v.log.With().
		Stringer("old_stop", &v.stopBoundary).
		Stringer("new_stop", &stopBoundary).
		Logger()

	err := v.validateStopChange(stopBoundary)
	if err != nil {
		log.Info().Err(err).Msg("cannot set stopHeight")
		return err
	}

	log.Info().Msg("new stop set")
	v.stopBoundary = stopBoundary

	return nil
}

var ErrCannotChangeStop = errors.New("cannot change stop control stopping parameters")

// validateStopChange verifies if the stop parameters can be changed
// returns the error with the reason if the parameters cannot be changed.
//
// Stop parameters cannot be changed if:
//  1. node is already stopped
//  2. stop parameters are immutable (due to them already affecting execution see
//     ShouldExecuteBlock)
//  3. stop parameters are already set by a different source and the new stop is later than
//     the existing one
//
// Expected error returns during normal operations:
//   - ErrCannotChangeStop: this indicates that new stop parameters cannot be set.
//
// Caller must acquire the lock.
func (v *VersionControl) validateStopChange(
	newStopBoundary stopBoundary,
) error {

	errf := func(reason string) error {
		return fmt.Errorf("%s: %w", reason, ErrCannotChangeStop)
	}

	if v.stopBoundary.immutable {
		return errf(
			fmt.Sprintf(
				"cannot update stopHeight, stopping commenced for %s",
				v.stopBoundary),
		)
	}

	if !v.stopBoundary.Set() {
		// if the current stop is no stop, we can always update
		return nil
	}

	// 1.
	// if one stop was set by the version beacon and the other one was manual
	// we can only update if the new stop is strictly earlier
	if newStopBoundary.StopBeforeHeight < v.stopBoundary.StopBeforeHeight {
		return nil

	}
	// this prevents users moving the stopHeight forward when a version newStopBoundary
	// is earlier, and prevents version beacons from moving the stopHeight forward
	// when a manual stop is earlier.
	return errf("cannot update stopHeight, " +
		"new stop height is later than the current one")
}

// GetStopBeforeHeight returns the upcoming stop height
func (v *VersionControl) GetStopBeforeHeight() uint64 {
	v.RLock()
	defer v.RUnlock()

	return v.stopBoundary.StopBeforeHeight
}

// blockFinalized is called when a block is marked as finalized
//
// Once finalization reached stopHeight we can be sure no other fork will be valid at
// this height, if this block's parent has been executed, we are safe to stop.
// This will happen during normal execution, where blocks are executed
// before they are finalized. However, it is possible that EN block computation
// progress can fall behind. In this case, we want to crash only after the execution
// reached the stopHeight.
func (v *VersionControl) blockFinalized(
	ctx irrecoverable.SignalerContext,
	h *flow.Header,
) {
	v.Lock()
	defer v.Unlock()

	// We already know the ID of the block that should be executed last nothing to do.
	// Node is stopping.
	if v.stopBoundary.stopAfterExecuting != flow.ZeroID {
		return
	}

	handleErr := func(err error) {
		v.log.Err(err).
			Stringer("block_id", h.ID()).
			Stringer("stop", &v.stopBoundary).
			Msg("Error in stop control BlockFinalized")

		ctx.Throw(err)
	}

	v.processNewVersionBeacons(ctx, h.Height)

	// we are not at the stop yet, nothing to do
	if h.Height < v.stopBoundary.StopBeforeHeight {
		return
	}

	parentID := h.ParentID

	if h.Height != v.stopBoundary.StopBeforeHeight {
		// we are past the stop. This can happen if stop was set before
		// last finalized block
		v.log.Warn().
			Uint64("finalization_height", h.Height).
			Stringer("block_id", h.ID()).
			Stringer("stop", &v.stopBoundary).
			Msg("Block finalization already beyond stop.")

		// Let's find the ID of the block that should be executed last
		// which is the parent of the block at the stopHeight
		finalizedID, err := v.headers.BlockIDByHeight(v.stopBoundary.StopBeforeHeight - 1)
		if err != nil {
			handleErr(fmt.Errorf("failed to get header by height: %w", err))
			return
		}
		parentID = finalizedID
	}

	v.stopBoundary.stopAfterExecuting = parentID

	v.log.Info().
		Stringer("block_id", h.ID()).
		Stringer("stop", &v.stopBoundary).
		Stringer("stop_after_executing", v.stopBoundary.stopAfterExecuting).
		Msgf("Found ID of the block that should be executed last")

	//TODO: should check if parent block is executed already

	// check if the parent block has been executed then stop right away
	//executed, err := state.IsParentExecuted(v.exeState, h)
	//if err != nil {
	//	handleErr(fmt.Errorf(
	//		"failed to check if the block has been executed: %w",
	//		err,
	//	))
	//	return
	//}
	//
	//if executed {
	//	// we already reached the point where we should stop
	//	v.stopExecution()
	//	return
	//}
}

// stopExecution stops the node execution and crashes the node if ShouldCrash is true.
// Caller must acquire the lock.
func (v *VersionControl) stopExecution() {
	log := v.log.With().
		Stringer("requested_stop", &v.stopBoundary).
		Uint64("last_executed_height", v.stopBoundary.StopBeforeHeight).
		Stringer("last_executed_id", v.stopBoundary.stopAfterExecuting).
		Logger()

	log.Warn().Msg("Stopping as finalization reached requested stop")

	log.Info().
		Dur("max-graceful-stop-duration", v.maxGracefulStopDuration).
		Msg("Attempting graceful stop as finalization reached requested stop")
	doneChan := v.unit.Done()
	select {
	case <-doneChan:
		log.Info().Msg("Engine gracefully stopped")
	case <-time.After(v.maxGracefulStopDuration):
		log.Info().
			Msg("Engine did not stop within max graceful stop duration")
	}
	log.Fatal().Msg("Crashing as finalization reached requested stop")
	return
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
func (v *VersionControl) processNewVersionBeacons(
	ctx irrecoverable.SignalerContext,
	height uint64,
) {
	// TODO: remove when we can guarantee that the node will always have a valid version
	if v.nodeVersion == nil {
		return
	}

	if v.versionBeacon != nil && v.versionBeacon.SealHeight >= height {
		// already processed this or a higher version beacon
		return
	}

	vb, err := v.versionBeacons.Highest(height)
	if err != nil {
		v.log.Err(err).
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
		v.log.Info().
			Uint64("height", height).
			Msg("No version beacon found for stop control")
		return
	}

	if v.versionBeacon != nil && v.versionBeacon.SealHeight >= vb.SealHeight {
		// we already processed this or a higher version beacon
		return
	}

	lg := v.log.With().
		Str("node_version", v.nodeVersion.String()).
		Str("beacon", vb.String()).
		Uint64("vb_seal_height", vb.SealHeight).
		Uint64("vb_sequence", vb.Sequence).Logger()

	// this is now the last handled version beacon
	v.versionBeacon = vb

	// this is a new version beacon check what boundary it sets
	stopHeight, err := v.getVersionBeaconStopHeight(vb)
	if err != nil {
		v.log.Err(err).
			Interface("version_beacon", vb).
			Msg("Failed to get stop height from version beacon")

		ctx.Throw(
			fmt.Errorf("failed to get stop height from version beacon: %w", err))
		return
	}

	lg.Info().
		Uint64("stop_height", stopHeight).
		Msg("New version beacon found")

	var newStop = stopBoundary{
		StopBeforeHeight: stopHeight,
	}

	err = v.setStopParameters(newStop)
	if err != nil {
		// This is just informational and is expected to sometimes happen during
		// normal operation. The causes for this are described here: validateStopChange.
		v.log.Info().
			Uint64("stop_height", stopHeight).
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
func (v *VersionControl) getVersionBeaconStopHeight(
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
		if v.nodeVersion.LessThan(*ver) {
			// we need to stop here
			return boundary.BlockHeight, nil
		}
	}

	// no stop boundary should be set
	return NoStopHeight, nil
}
