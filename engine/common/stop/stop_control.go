package stop

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/irrecoverable"

	"github.com/onflow/flow-go/engine/common/version"
)

// StopControl is responsible for managing the stopping behavior of the node
// when an incompatible block height is encountered.
type StopControl struct {
	log zerolog.Logger

	// incompatibleBlockHeight is the height of the block that is incompatible with the current node version.
	incompatibleBlockHeight *atomic.Uint64
	// updatedVersion is the expected node version to continue working with new blocks.
	updatedVersion *atomic.String
}

// NewStopControl creates a new StopControl instance.
func NewStopControl(
	log zerolog.Logger,
	versionControl *version.VersionControl,
) *StopControl {
	sc := &StopControl{
		log: log.With().
			Str("component", "stop_control").
			Logger(),
		incompatibleBlockHeight: atomic.NewUint64(0),
		updatedVersion:          atomic.NewString(""),
	}

	if versionControl != nil {
		// Subscribe for version updates
		versionControl.AddVersionUpdatesConsumer(sc.OnVersionUpdate)
	} else {
		sc.log.Info().
			Msg("version control is nil")
	}

	return sc
}

// updateVersionData sets new version data
func (sc *StopControl) updateVersionData(height uint64, semver string) {
	sc.incompatibleBlockHeight.Store(height)
	sc.updatedVersion.Store(semver)
}

// OnVersionUpdate is called when a version update occurs.
//
// It updates the incompatible block height and the expected node version
// based on the provided height and semver.
func (sc *StopControl) OnVersionUpdate(height uint64, version *semver.Version) {
	// If the version was updated, store new version information
	if version != nil {
		sc.log.Info().
			Uint64("height", height).
			Str("semver", version.String()).
			Msg("Received version update")

		sc.updateVersionData(height, version.String())
		return
	}

	// If semver is 0, but notification was received, this means that the version update was deleted.
	sc.updateVersionData(0, "")
}

// OnProcessedBlock is called when need to check processed block for compatibility with current node.
func (sc *StopControl) OnProcessedBlock(ctx irrecoverable.SignalerContext, height uint64) {
	incompatibleBlockHeight := sc.incompatibleBlockHeight.Load()
	if height >= incompatibleBlockHeight {
		ctx.Throw(fmt.Errorf("processed block at height %d is incompatible with the current node version, please upgrade to version %s starting from block height %d",
			height, sc.updatedVersion.Load(), incompatibleBlockHeight))
	}
}
