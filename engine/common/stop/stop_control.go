package stop

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"

	"github.com/onflow/flow-go/engine/common/version"
)

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

	// Notifier for new processed block height
	processedHeightNotifier engine.Notifier

	// Stores latest highest processed block height
	lastProcessedHeight counters.StrictMonotonousCounter
}

// NewStopControl creates a new StopControl instance.
//
// Parameters:
//   - log: The logger used for logging.
//   - versionControl: The version control used to track node version updates.
//
// Returns:
//   - A pointer to the newly created StopControl instance.
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
		lastProcessedHeight:     counters.NewMonotonousCounter(0),
		processedHeightNotifier: engine.NewNotifier(),
	}

	sc.cm = component.NewComponentManagerBuilder().
		AddWorker(sc.processEvents).
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

// updateVersionData sets new version data.
//
// Parameters:
//   - height: The block height that is incompatible with the current node version.
//   - semver: The new semantic version string that is expected for compatibility.
func (sc *StopControl) updateVersionData(height uint64, semver string) {
	sc.incompatibleBlockHeight.Store(height)
	sc.updatedVersion.Store(semver)
}

// OnVersionUpdate is called when a version update occurs.
//
// It updates the incompatible block height and the expected node version
// based on the provided height and semver.
//
// Parameters:
//   - height: The block height that is incompatible with the current node version.
//   - version: The new semantic version object that is expected for compatibility.
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
//
// Parameters:
//   - ctx: The context used to signal an irrecoverable error if the processed block is incompatible.
func (sc *StopControl) OnProcessedBlock(ctx irrecoverable.SignalerContext) {
	incompatibleBlockHeight := sc.incompatibleBlockHeight.Load()
	newHeight := sc.lastProcessedHeight.Value()
	if newHeight >= incompatibleBlockHeight {
		ctx.Throw(fmt.Errorf("processed block at height %d is incompatible with the current node version, please upgrade to version %s starting from block height %d",
			newHeight, sc.updatedVersion.Load(), incompatibleBlockHeight))
	}
}

// updateProcessedHeight updates the last processed height and triggers notifications.
//
// Parameters:
//   - height: The height of the latest processed block.
func (sc *StopControl) updateProcessedHeight(height uint64) {
	if sc.lastProcessedHeight.Set(height) {
		sc.processedHeightNotifier.Notify()
	}
}

// RegisterHeightRecorder registers an execution data height recorder with the StopControl.
//
// Parameters:
//   - recorder: The execution data height recorder to register.
func (sc *StopControl) RegisterHeightRecorder(recorder execution_data.ProcessedHeightRecorder) {
	if recorder != nil {
		recorder.AddHeightUpdatesConsumer(sc.updateProcessedHeight)
		sc.registeredHeightRecorders = append(sc.registeredHeightRecorders, recorder)
	}
}

// processEvents processes incoming events related to block heights and version updates.
//
// Parameters:
//   - ctx: The context used to handle irrecoverable errors.
//   - ready: A function to signal that the component is ready to start processing events.
func (sc *StopControl) processEvents(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sc.processedHeightNotifier.Channel():
			sc.OnProcessedBlock(ctx)
		}
	}
}
