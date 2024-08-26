package stop

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type VersionMetadata struct {
	// incompatibleBlockHeight is the height of the block that is incompatible with the current node version.
	incompatibleBlockHeight uint64
	// updatedVersion is the expected node version to continue working with new blocks.
	updatedVersion string
}

// StopControl is responsible for managing the stopping behavior of the node
// when an incompatible block height is encountered.
type StopControl struct {
	component.Component
	cm *component.ComponentManager

	log zerolog.Logger

	versionData *atomic.Pointer[VersionMetadata]

	// Notifier for new processed block height
	processedHeightChannel chan uint64

	// Stores latest processed block height
	lastProcessedHeight counters.StrictMonotonousCounter
}

// NewStopControl creates a new StopControl instance.
//
// Parameters:
//   - log: The logger used for logging.
//
// Returns:
//   - A pointer to the newly created StopControl instance.
func NewStopControl(
	log zerolog.Logger,
) *StopControl {
	sc := &StopControl{
		log: log.With().
			Str("component", "stop_control").
			Logger(),
		lastProcessedHeight:    counters.NewMonotonousCounter(0),
		versionData:            atomic.NewPointer[VersionMetadata](nil),
		processedHeightChannel: make(chan uint64),
	}

	sc.cm = component.NewComponentManagerBuilder().
		AddWorker(sc.processEvents).
		Build()
	sc.Component = sc.cm

	return sc
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

		sc.versionData.Store(&VersionMetadata{
			incompatibleBlockHeight: height,
			updatedVersion:          version.String(),
		})
		return
	}

	// If semver is 0, but notification was received, this means that the version update was deleted.
	sc.versionData.Store(nil)
}

// onProcessedBlock is called when a new block is processed block.
// when the last compatible block is processed, the StopControl will cause the node to crash
//
// Parameters:
//   - ctx: The context used to signal an irrecoverable error.
func (sc *StopControl) onProcessedBlock(ctx irrecoverable.SignalerContext) {
	versionData := sc.versionData.Load()
	if versionData == nil {
		return
	}

	newHeight := sc.lastProcessedHeight.Value()
	if newHeight >= versionData.incompatibleBlockHeight-1 {
		ctx.Throw(fmt.Errorf("processed block at height %d is incompatible with the current node version, please upgrade to version %s starting from block height %d",
			newHeight, versionData.updatedVersion, versionData.incompatibleBlockHeight))
	}
}

// updateProcessedHeight updates the last processed height and triggers notifications.
//
// Parameters:
//   - height: The height of the latest processed block.
func (sc *StopControl) updateProcessedHeight(height uint64) {
	sc.processedHeightChannel <- height
}

// RegisterHeightRecorder registers an execution data height recorder with the StopControl.
//
// Parameters:
//   - recorder: The execution data height recorder to register.
func (sc *StopControl) RegisterHeightRecorder(recorder execution_data.ProcessedHeightRecorder) {
	recorder.SetHeightUpdatesConsumer(sc.updateProcessedHeight)
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
		case height, ok := <-sc.processedHeightChannel:
			if !ok {
				return
			}
			if sc.lastProcessedHeight.Set(height) {
				sc.onProcessedBlock(ctx)
			}
		}
	}
}
