package stop

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

// OnVersionUpdate which receives the signals from the VersionControl. This method is responsible for receiving notifications about the next version update event
// OnProcessedBlock which receives a block header/height and is called by other subsystems on the node when they finished handling a block. This is equivalent to StopControl.OnBlockExecuted, and on the AN would subscribe to events from the indexer engine.
type StopControl struct {
	// Noop implements the protocol.Consumer interface with no operations.
	events.Noop
	component.Component

	log            zerolog.Logger
	versionControl *version.VersionControl
}

var (
	_ component.Component = (*StopControl)(nil)
	_ protocol.Consumer   = (*StopControl)(nil)
)

// NewStopControl creates a new StopControl instance.
//
// We currently have no strong guarantee that the node version is a valid semver.
// See build.SemverV2 for more details. That is why nil is a valid input for node version.
func NewStopControl(
	log zerolog.Logger,
	versionControl *version.VersionControl,
) (*StopControl, error) {
	sc := &StopControl{
		log: log.With().
			Str("component", "stop_control").
			Logger(),
		versionControl: versionControl,
	}

	versionControl.AddVersionUpdatesConsumer(sc.OnVersionUpdate)

	// Setup component manager for handling worker functions.
	cm := component.NewComponentManagerBuilder()

	sc.Component = cm.Build()

	return sc, nil
}

func (sc *StopControl) BlockFinalized(h *flow.Header) {
	sc.OnProcessedBlock(h)
}

func (sc *StopControl) OnVersionUpdate(height uint64, semver string) {
}

func (sc *StopControl) OnProcessedBlock(header *flow.Header) {
}
