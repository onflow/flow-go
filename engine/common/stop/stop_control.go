package stop

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"

	"github.com/onflow/flow-go/engine/common/version"
)

// ErrNoRegisteredHeightRecorders represents an error indicating that pruner did not register any execution data height recorders.
// This error occurs when the pruner attempts to perform operations that require
// at least one registered height recorder, but none are found.
var ErrNoRegisteredHeightRecorders = errors.New("no registered height recorders")

// StopControl is responsible for managing the stopping behavior of the node
// when an incompatible block height is encountered.
type StopControl struct {
	component.Component
	cm *component.ComponentManager

	log zerolog.Logger

	// incompatibleBlockHeight is the height of the block that is incompatible with the current node version.
	incompatibleBlockHeight *atomic.Uint64
	// updatedVersion is the expected node version to continue working with new blocks.
	updatedVersion *atomic.String

	registeredHeightRecorders []execution_data.ProcessedHeightRecorder
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

	sc.cm = component.NewComponentManagerBuilder().
		AddWorker(sc.loop).
		Build()
	sc.Component = sc.cm

	if versionControl != nil {
		// Subscribe for version updates
		versionControl.AddVersionUpdatesConsumer(sc.OnVersionUpdate)
	} else {
		sc.log.Info().
			Msg("version control is uninitialized")
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

// RegisterHeightRecorder registers an execution data height recorder with the StopControl.
//
// Parameters:
//   - recorder: The execution data height recorder to register.
func (sc *StopControl) RegisterHeightRecorder(recorder execution_data.ProcessedHeightRecorder) {
	sc.registeredHeightRecorders = append(sc.registeredHeightRecorders, recorder)
}

func (sc *StopControl) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	//TODO: Check defaults
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lowestHeight, err := sc.lowestRecordersHeight()
			if err == nil || !errors.Is(err, ErrNoRegisteredHeightRecorders) {
				sc.OnProcessedBlock(ctx, lowestHeight)
			}

		}
	}
}

// lowestRecordersHeight returns the lowest height among all registered
// height recorders.
//
// This function iterates over all registered height recorders to determine
// the smallest of complete height recorded. If no height recorders are registered, it
// returns an error.
//
// Expected errors during normal operation:
// - ErrNoRegisteredHeightRecorders: if no height recorders are registered.
func (sc *StopControl) lowestRecordersHeight() (uint64, error) {
	if len(sc.registeredHeightRecorders) == 0 {
		return 0, ErrNoRegisteredHeightRecorders
	}

	lowestHeight := uint64(math.MaxUint64)
	for _, recorder := range sc.registeredHeightRecorders {
		height := recorder.HighestCompleteHeight()
		if height < lowestHeight {
			lowestHeight = height
		}
	}
	return lowestHeight, nil
}
