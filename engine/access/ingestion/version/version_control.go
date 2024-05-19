package version

import (
	"fmt"
	"math"
	"sync"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	psEvents "github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
)

type VersionControlConsumer func(height uint64, semver string)

var AnyHeight = uint64(math.MaxUint64)

// VersionControl TODO
type VersionControl struct {
	// Version control needs to consume only BlockFinalized events.
	// adding psEvents.Noop makes it a protocol.Consumer without unnecessary implementations
	psEvents.Noop
	sync.RWMutex
	component.Component

	log            zerolog.Logger
	headers        storage.Headers
	versionBeacons storage.VersionBeacons

	// nodeVersion could be nil right now. See NewVersionControl.
	nodeVersion *semver.Version
	// last seen version beacon, used to detect version beacon changes
	versionBeacon *flow.SealedVersionBeacon

	// array of consumers
	consumers []VersionControlConsumer

	blockFinalizedChan chan *flow.Header

	startHeight uint64
	endHeight   uint64
}

var _ protocol.Consumer = (*VersionControl)(nil)
var _ component.Component = (*VersionControl)(nil)

// NewVersionControl creates new VersionControl.
//
// We currently have no strong guarantee that the node version is a valid semver.
// See build.SemverV2 for more details. That is why nil is a valid input for node version
// without a node version, the stop control can still be used for manual stopping.
func NewVersionControl(
	log zerolog.Logger,
	headers storage.Headers,
	versionBeacons storage.VersionBeacons,
	nodeVersion *semver.Version,
	latestFinalizedBlock *flow.Header,
) *VersionControl {
	// We should not miss block finalized events, and we should be able to handle them
	// faster than they are produced anyway.
	blockFinalizedChan := make(chan *flow.Header, 1000)

	vc := &VersionControl{
		log: log.With().
			Str("component", "version_control").
			Logger(),

		blockFinalizedChan: blockFinalizedChan,

		headers:        headers,
		nodeVersion:    nodeVersion,
		versionBeacons: versionBeacons,
		startHeight:    AnyHeight,
		endHeight:      AnyHeight,
	}

	if vc.nodeVersion != nil {
		log = log.With().
			Stringer("node_version", vc.nodeVersion).
			Logger()
	}

	log.Info().Msgf("Created")

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(vc.processEvents)
	cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		vc.checkInitialVersionBeacon(ctx, ready, latestFinalizedBlock)
	})

	vc.Component = cm.Build()

	// TODO: handle version beacon already indicating a stop
	// right now the stop will happen on first BlockFinalized
	// which is fine, but ideally we would stop right away.

	return vc
}

// BlockFinalized is called when a block is finalized.
//
// This is a protocol event consumer. See protocol.Consumer.
func (v *VersionControl) BlockFinalized(h *flow.Header) {
	v.blockFinalizedChan <- h
}

func (v *VersionControl) CompatibleAtBlock(height uint64) bool {
	return (v.startHeight == AnyHeight || height >= v.startHeight) &&
		(v.endHeight == AnyHeight || height <= v.endHeight)
}

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

	vb, err := v.versionBeacons.Highest(latestFinalizedBlock.Height)
	if err != nil {
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
			Uint64("height", latestFinalizedBlock.Height).
			Msg("No version beacon found for version control")
		return
	}

	v.versionBeacon = vb

	lastCompatibleHeight := latestFinalizedBlock.Height

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

		if ver.LessThan(*v.nodeVersion) {
			v.startHeight = lastCompatibleHeight
			return
		}

		lastCompatibleHeight = boundary.BlockHeight
	}
}

// blockFinalized is called when a block is marked as finalized
func (v *VersionControl) blockFinalized(
	ctx irrecoverable.SignalerContext,
	h *flow.Header,
) {
	v.Lock()
	defer v.Unlock()

	// TODO: remove when we can guarantee that the node will always have a valid version
	if v.nodeVersion == nil {
		return
	}

	if v.versionBeacon != nil && v.versionBeacon.SealHeight >= h.Height {
		// already processed this or a higher version beacon
		return
	}

	vb, err := v.versionBeacons.Highest(h.Height)
	if err != nil {
		v.log.Err(err).
			Uint64("height", h.Height).
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
			Uint64("height", h.Height).
			Msg("No version beacon found for version control")
		return
	}

	if v.versionBeacon != nil && v.versionBeacon.SealHeight >= vb.SealHeight {
		// we already processed this or a higher version beacon
		return
	}

	lastCompatibleHeight := h.Height

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
			v.endHeight = lastCompatibleHeight

			for _, consumer := range v.consumers {
				consumer(v.endHeight, ver.String())
			}

			break
		}

		lastCompatibleHeight = boundary.BlockHeight
	}
}
