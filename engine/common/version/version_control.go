package version

import (
	"errors"
	"fmt"
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
// There are two possible notifications options:
// - A new or updated version will have a height and a semantic version at that height.
// - A deleted version will have the previous height and nil semantic version, indicating that the update was deleted.
type VersionControlConsumer func(height uint64, version *semver.Version)

// NoHeight represents the maximum possible height for blocks.
var NoHeight = uint64(0)

// defaultCompatibilityOverrides stores the list of version compatibility overrides.
// version beacon events who's Major.Minor.Patch version match an entry in this map will be ignored.
//
// IMPORTANT: only add versions to this list if you are certain that the cadence and fvm changes
// deployed during the HCU are backwards compatible for scripts.
var defaultCompatibilityOverrides = map[string]struct{}{
	"0.37.11": {}, // mainnet, testnet
	"0.37.17": {}, // mainnet, testnet
	"0.37.18": {}, // testnet only
	"0.37.20": {}, // mainnet, testnet
	"0.37.22": {}, // mainnet, testnet
	"0.37.26": {}, // mainnet, testnet
	"0.38.1":  {}, // testnet only
	"0.38.2":  {}, // mainnet, testnet
	"0.38.3":  {}, // mainnet, testnet
	"0.40.0":  {}, // mainnet, testnet
	"0.41.0":  {}, // mainnet, testnet
	"0.41.4":  {}, // mainnet, testnet
	"0.42.0":  {}, // mainnet, testnet
	"0.42.1":  {}, // mainnet, testnet
	"0.42.3":  {}, // mainnet, testnet
	"0.43.1":  {}, // testnet only
	"0.44.0":  {}, // mainnet, testnet
	"0.44.1":  {}, // mainnet, testnet
	"0.44.7":  {}, // mainnet, testnet
	"0.44.10": {}, // mainnet, testnet
	"0.44.14": {}, // mainnet, testnet
	"0.44.15": {}, // mainnet, testnet
	"0.44.16": {}, // mainnet, testnet
	"0.44.17": {}, // mainnet, testnet
	"0.44.18": {}, // mainnet, testnet
	"0.45.0":  {}, // mainnet, testnet
	"0.46.0":  {}, // mainnet, testnet
	"0.47.0":  {}, // mainnet, testnet
}

// VersionControl manages the version control system for the node.
// It consumes BlockFinalized events and updates the node's version control based on the latest version beacon.
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

	finalizedHeight counters.StrictMonotonicCounter

	// lastProcessedHeight the last handled block height
	lastProcessedHeight *atomic.Uint64

	// sealedRootBlockHeight the last sealed block height when node bootstrapped
	sealedRootBlockHeight *atomic.Uint64

	// startHeight and endHeight define the height boundaries for version compatibility.
	startHeight *atomic.Uint64
	endHeight   *atomic.Uint64

	// compatibilityOverrides stores the list of version compatibility overrides.
	// version beacon events who's Major.Minor.Patch version match an entry in this map will be ignored.
	compatibilityOverrides map[string]struct{}

	// overridesLogSuppression stores the list of version compatibility overrides that have been logged.
	// this is used to avoid emitting logs during every check when a version is overridden.
	overridesLogSuppression map[string]struct{}
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
	sealedRootBlockHeight uint64,
	latestFinalizedBlockHeight uint64,
) (*VersionControl, error) {

	vc := &VersionControl{
		log: log.With().
			Str("component", "version_control").
			Logger(),

		nodeVersion:             nodeVersion,
		versionBeacons:          versionBeacons,
		sealedRootBlockHeight:   atomic.NewUint64(sealedRootBlockHeight),
		lastProcessedHeight:     atomic.NewUint64(latestFinalizedBlockHeight),
		finalizedHeight:         counters.NewMonotonicCounter(latestFinalizedBlockHeight),
		finalizedHeightNotifier: engine.NewNotifier(),
		startHeight:             atomic.NewUint64(NoHeight),
		endHeight:               atomic.NewUint64(NoHeight),
		compatibilityOverrides:  defaultCompatibilityOverrides,
		overridesLogSuppression: make(map[string]struct{}),
	}

	if vc.nodeVersion == nil {
		return nil, fmt.Errorf("version control node version is empty")
	}

	vc.log.Info().
		Stringer("node_version", vc.nodeVersion).
		Msg("system initialized")

	// Setup component manager for handling worker functions.
	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(vc.processEvents)
	cm.AddWorker(vc.checkInitialVersionBeacon)

	vc.Component = cm.Build()

	return vc, nil
}

// checkInitialVersionBeacon checks the initial version beacon at the latest finalized block.
// It ensures the component is not ready until the initial version beacon is checked.
func (v *VersionControl) checkInitialVersionBeacon(
	ctx irrecoverable.SignalerContext,
	ready component.ReadyFunc,
) {
	err := v.initBoundaries(ctx)
	if err == nil {
		ready()
	}
}

// initBoundaries initializes the version boundaries for version control.
//
// It searches through version beacons to find the start and end block heights
// for the current node version. The search continues until the start height
// is found or until the sealed root block height is reached.
//
// Returns an error when could not get the highest version beacon event
func (v *VersionControl) initBoundaries(
	ctx irrecoverable.SignalerContext,
) error {
	sealedRootBlockHeight := v.sealedRootBlockHeight.Load()
	latestHeight := v.lastProcessedHeight.Load()
	processedHeight := latestHeight

	for {
		vb, err := v.versionBeacons.Highest(processedHeight)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			ctx.Throw(fmt.Errorf("failed to get highest version beacon for version control: %w", err))
			return err
		}

		if vb == nil {
			// no version beacon found
			// this is only expected when a node starts up on a network that has never had a version beacon event.
			v.log.Info().
				Uint64("height", processedHeight).
				Msg("No initial version beacon found")

			return nil
		}

		// version boundaries are sorted by blockHeight in ascending order
		// the first version greater than the node's is the version transition height
		for i := len(vb.VersionBoundaries) - 1; i >= 0; i-- {
			boundary := vb.VersionBoundaries[i]

			ver, err := boundary.Semver()
			// this should never happen as we already validated the version beacon
			// when indexing it
			if err != nil || ver == nil {
				if err == nil {
					err = fmt.Errorf("boundary semantic version is nil")
				}
				ctx.Throw(fmt.Errorf("failed to parse semver during version control setup: %w", err))
				return err
			}
			processedHeight = vb.SealHeight - 1

			if v.isOverridden(ver) {
				continue
			}

			if ver.Compare(*v.nodeVersion) <= 0 {
				v.startHeight.Store(boundary.BlockHeight)
				v.log.Info().
					Uint64("startHeight", boundary.BlockHeight).
					Msg("Found start block height")
				// This is the lowest compatible height for this node version, stop search immediately
				return nil
			} else {
				v.endHeight.Store(boundary.BlockHeight - 1)
				v.log.Info().
					Uint64("endHeight", boundary.BlockHeight-1).
					Msg("Found end block height")
			}
		}

		// The search should continue until we find the start height or reach the sealed root block height
		if v.startHeight.Load() == NoHeight && processedHeight <= sealedRootBlockHeight {
			v.log.Info().
				Uint64("processedHeight", processedHeight).
				Uint64("sealedRootBlockHeight", sealedRootBlockHeight).
				Msg("No start version beacon event found")
			return nil
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
	// Check, if the height smaller than sealed root block height. If so, return an error indicating that the height is unhandled.
	sealedRootHeight := v.sealedRootBlockHeight.Load()
	if height < sealedRootHeight {
		return false, fmt.Errorf("could not check compatibility for height %d: the provided height is smaller than sealed root height %d: %w", height, sealedRootHeight, ErrOutOfRange)
	}

	// Check if the height is greater than the last handled block height. If so, return an error indicating that the height is unhandled.
	lastProcessedHeight := v.lastProcessedHeight.Load()
	if height > lastProcessedHeight {
		return false, fmt.Errorf("could not check compatibility for height %d: last handled height is %d: %w", height, lastProcessedHeight, ErrOutOfRange)
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
				Msg("Failed to get highest version beacon")

			ctx.Throw(fmt.Errorf("failed to get highest version beacon for version control: %w", err))
			return
		}

		v.lastProcessedHeight.Store(height)

		if vb == nil {
			// no version beacon found
			// this is only expected when a node starts up on a network that has never had a version beacon event.
			v.log.Debug().
				Uint64("height", height).
				Msg("No version beacon found at height")
			continue
		}

		previousEndHeight := v.endHeight.Load()

		if previousEndHeight != NoHeight && height > previousEndHeight {
			// Stop here since it's outside our compatible range
			return
		}

		newEndHeight := NoHeight
		// version boundaries are sorted by blockHeight in ascending order
		for _, boundary := range vb.VersionBoundaries {
			ver, err := boundary.Semver()
			if err != nil || ver == nil {
				if err == nil {
					err = fmt.Errorf("boundary semantic version is nil")
				}
				// this should never happen as we already validated the version beacon
				// when indexing it
				ctx.Throw(fmt.Errorf("failed to parse semver: %w", err))
				return
			}

			if v.isOverridden(ver) {
				continue
			}

			if ver.Compare(*v.nodeVersion) > 0 {
				newEndHeight = boundary.BlockHeight - 1

				for _, consumer := range v.consumers {
					consumer(boundary.BlockHeight, ver)
				}

				break
			}
		}

		v.endHeight.Store(newEndHeight)

		// Check if previous version was deleted. If yes, notify consumers about deletion
		if previousEndHeight != NoHeight && newEndHeight == NoHeight {
			for _, consumer := range v.consumers {
				// Note: notifying for the boundary height, which is end height + 1
				consumer(previousEndHeight+1, nil)
			}
		}
	}
}

// StartHeight return the first block that the version supports.
// Start height is the sealed root block if there is no start boundary in the current spork.
func (v *VersionControl) StartHeight() uint64 {
	startHeight := v.startHeight.Load()
	sealedRootHeight := v.sealedRootBlockHeight.Load()

	// in case no start boundary in the current spork
	if startHeight == NoHeight {
		startHeight = sealedRootHeight
	}

	return max(startHeight, sealedRootHeight)
}

// EndHeight return the last block that the version supports.
// End height is the last processed height if there is no end boundary in the current spork.
func (v *VersionControl) EndHeight() uint64 {
	endHeight := v.endHeight.Load()

	// in case no end boundary in the current spork
	if endHeight == NoHeight {
		endHeight = v.lastProcessedHeight.Load()
	}

	return endHeight
}

// isOverridden checks if the version is overridden by the compatibility overrides and can be ignored.
func (v *VersionControl) isOverridden(ver *semver.Version) bool {
	normalizedVersion := semver.Version{
		Major: ver.Major,
		Minor: ver.Minor,
		Patch: ver.Patch,
	}.String()

	if _, ok := v.compatibilityOverrides[normalizedVersion]; !ok {
		return false
	}

	// only log the suppression once per version
	if _, ok := v.overridesLogSuppression[normalizedVersion]; !ok {
		v.overridesLogSuppression[normalizedVersion] = struct{}{}
		v.log.Info().
			Str("event_version", ver.String()).
			Str("override_version", normalizedVersion).
			Msg("ignoring version beacon event matching compatibility override")
	}

	return true
}
