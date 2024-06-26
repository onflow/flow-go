package version

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	psEvents "github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
)

// ErrOutOfRange indicates that height is higher than last handled block height
var ErrOutOfRange = errors.New("height is out of range")

// VersionControlConsumer defines a function type that consumes version control updates.
// It is called with the block height and the corresponding semantic version.
type VersionControlConsumer func(height uint64, semver string)

// NoHeight represents the maximum possible height for blocks.
var NoHeight = uint64(math.MaxUint64)

// VersionControl manages the version control system for the node.
// It consumes BlockFinalized events and updates the node's version control based on the latest version beacon.
//
// VersionControl implements the protocol.Consumer and component.Component interfaces.
type VersionControl struct {
	// Noop implements the protocol.Consumer interface with no operations.
	psEvents.Noop
	sync.Mutex
	component.Component

	log zerolog.Logger
	// Storage
	versionBeacons storage.VersionBeacons

	// nodeVersion stores the node's current version.
	// It could be nil if the node version is not available.
	nodeVersion *semver.Version

	// consumers stores the list of consumers for version updates.
	consumers []VersionControlConsumer

	// Notifier for new finalized block height
	finalizedHeightNotifier engine.Notifier

	finalizedHeight counters.StrictMonotonousCounter

	// lastProcessedHeight the last handled block height
	lastProcessedHeight *atomic.Uint64

	// startHeight and endHeight define the height boundaries for version compatibility.
	startHeight *atomic.Uint64
	endHeight   *atomic.Uint64
}

var _ protocol.Consumer = (*VersionControl)(nil)
var _ component.Component = (*VersionControl)(nil)

// NewVersionControl creates a new VersionControl instance.
//
// We currently have no strong guarantee that the node version is a valid semver.
// See build.SemverV2 for more details. That is why nil is a valid input for node version.
func NewVersionControl(
	log zerolog.Logger,
	versionBeacons storage.VersionBeacons,
	nodeVersion *semver.Version,
	finalizedRootBlockHeight uint64,
	latestFinalizedBlockHeight uint64,
) (*VersionControl, error) {

	vc := &VersionControl{
		log: log.With().
			Str("component", "version_control").
			Logger(),

		nodeVersion:             nodeVersion,
		versionBeacons:          versionBeacons,
		lastProcessedHeight:     atomic.NewUint64(latestFinalizedBlockHeight),
		finalizedHeight:         counters.NewMonotonousCounter(latestFinalizedBlockHeight),
		finalizedHeightNotifier: engine.NewNotifier(),
		startHeight:             atomic.NewUint64(NoHeight),
		endHeight:               atomic.NewUint64(NoHeight),
	}

	if vc.nodeVersion == nil {
		return nil, fmt.Errorf("version control node version is empty")
	}

	log.Info().
		Stringer("node_version", vc.nodeVersion).
		Msg("system initialized")

	// Setup component manager for handling worker functions.
	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(vc.processEvents)
	cm.AddWorker(vc.checkInitialVersionBeacon(finalizedRootBlockHeight))

	vc.Component = cm.Build()

	return vc, nil
}

// checkInitialVersionBeacon checks the initial version beacon at the latest finalized block.
// It ensures the component is not ready until the initial version beacon is checked.
func (v *VersionControl) checkInitialVersionBeacon(
	finalizedRootBlockHeight uint64,
) func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	return func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		// component is not ready until we checked the initial version beacon
		defer ready()

		processedHeight := v.lastProcessedHeight.Load()

		for {
			vb, err := v.versionBeacons.Highest(processedHeight)
			if err != nil {
				ctx.Throw(
					fmt.Errorf(
						"failed to get highest version beacon for version control: %w",
						err))
				return
			}

			if vb == nil {
				// no version beacon found
				// this is unexpected on a live network as there should always be at least the
				// starting version beacon, but not fatal.
				// It can happen on new or test networks if the node starts before bootstrap is finished.
				// TODO: remove when we can guarantee that there will always be a version beacon
				v.log.Info().
					Uint64("height", processedHeight).
					Msg("No version beacon found for version control")
				return
			}

			// version boundaries are sorted by version
			for i := len(vb.VersionBoundaries) - 1; i >= 0; i-- {
				boundary := vb.VersionBoundaries[i]

				ver, err := boundary.Semver()
				if err != nil || ver == nil {
					// this should never happen as we already validated the version beacon
					// when indexing it
					ctx.Throw(
						fmt.Errorf(
							"failed to parse semver during version control setup: %w",
							err))
					return
				}

				compResult := ver.Compare(*v.nodeVersion)
				processedHeight = boundary.BlockHeight - 1

				if compResult <= 0 {
					v.startHeight.Store(boundary.BlockHeight)
					v.log.Info().
						Uint64("startHeight", boundary.BlockHeight).
						Msg("Found start block height")
					// This is the lowest compatible height for this node version, stop search immediately
					return
				} else if compResult > 0 {
					v.endHeight.Store(boundary.BlockHeight - 1)
					v.log.Info().
						Uint64("endHeight", boundary.BlockHeight-1).
						Msg("Found end block height")
				}
			}

			// The search should continue until we find the start height or reach the finalized root block height
			if v.startHeight.Load() == NoHeight && processedHeight <= finalizedRootBlockHeight {
				v.log.Info().
					Uint64("processedHeight", processedHeight).
					Uint64("finalizedRootBlockHeight", finalizedRootBlockHeight).
					Msg("No start version beacon event found")
				return
			}
		}
	}
}

// BlockFinalized is called when a block is finalized.
// It implements the protocol.Consumer interface.
func (v *VersionControl) BlockFinalized(h *flow.Header) {
	if v.finalizedHeight.Set(h.Height) {
		v.finalizedHeightNotifier.Notify()
	}
}

// CompatibleAtBlock checks if the node's version is compatible at a given block height.
// It returns true if the node's version is compatible within the specified height range.
// Returns expected errors:
// - ErrOutOfRange if incoming block height is higher that last handled block height
func (v *VersionControl) CompatibleAtBlock(height uint64) (bool, error) {
	// Check if the height is greater than the last handled block height. If so, return an error indicating that the height is unhandled.
	if height > v.lastProcessedHeight.Load() {
		return false, fmt.Errorf("could not check compatibility for height %d: last handled height is %d: %w", height, v.lastProcessedHeight.Load(), ErrOutOfRange)
	}

	startHeight := v.startHeight.Load()
	// Check if the start height is set and the height is less than the start height. If so, return false indicating that the height is not compatible.
	if startHeight != NoHeight && height < startHeight {
		return false, nil
	}

	endHeight := v.endHeight.Load()
	// Check if the end height is set and the height is greater than the end height. If so, return false indicating that the height is not compatible.
	if endHeight != NoHeight && height > endHeight {
		return false, nil
	}

	// If none of the above conditions are met, the height is compatible.
	return true, nil
}

// AddVersionUpdatesConsumer adds a consumer for version update events.
func (v *VersionControl) AddVersionUpdatesConsumer(consumer VersionControlConsumer) {
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
		case <-v.finalizedHeightNotifier.Channel():
			v.blockFinalized(ctx, v.finalizedHeight.Value())
		}
	}
}

// blockFinalized processes a block finalized event and updates the version control state.
func (v *VersionControl) blockFinalized(
	ctx irrecoverable.SignalerContext,
	newFinalizedHeight uint64,
) {
	lastProcessedHeight := v.lastProcessedHeight.Load()
	if lastProcessedHeight >= newFinalizedHeight {
		// already processed this or a higher version beacon
		return
	}

	for height := lastProcessedHeight + 1; height <= newFinalizedHeight; height++ {
		vb, err := v.versionBeacons.Highest(height)
		if err != nil {
			v.log.Err(err).
				Uint64("height", height).
				Msg("Failed to get highest version beacon for version control")

			ctx.Throw(
				fmt.Errorf(
					"failed to get highest version beacon for version control: %w",
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
				Msg("No version beacon found for version control")
			continue
		}

		v.lastProcessedHeight.Store(height)

		// version boundaries are sorted by version
		for _, boundary := range vb.VersionBoundaries {
			ver, err := boundary.Semver()
			if err != nil || ver == nil {
				// this should never happen as we already validated the version beacon
				// when indexing it
				ctx.Throw(
					fmt.Errorf(
						"failed to parse semver: %w",
						err))
				return
			}

			if ver.Compare(*v.nodeVersion) > 0 {
				v.endHeight.Store(boundary.BlockHeight - 1)

				for _, consumer := range v.consumers {
					consumer(boundary.BlockHeight, ver.String())
				}

				break
			}
		}
	}
}
