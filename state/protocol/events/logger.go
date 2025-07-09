package events

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type EventLogger struct {
	Noop   // satisfy protocol events consumer interface
	logger zerolog.Logger
}

var _ protocol.Consumer = (*EventLogger)(nil)

func NewEventLogger(logger zerolog.Logger) *EventLogger {
	return &EventLogger{
		logger: logger.With().Str("module", "protocol_events_logger").Logger(),
	}
}

func (p EventLogger) EpochTransition(newEpochCounter uint64, header *flow.Header) {
	p.logger.Info().Uint64("newEpochCounter", newEpochCounter).
		Uint64("height", header.Height).
		Uint64("view", header.View).
		Msg("epoch transition")
}

func (p EventLogger) EpochSetupPhaseStarted(currentEpochCounter uint64, header *flow.Header) {
	p.logger.Info().Uint64("currentEpochCounter", currentEpochCounter).
		Uint64("height", header.Height).
		Uint64("view", header.View).
		Msg("epoch setup phase started")
}

func (p EventLogger) EpochCommittedPhaseStarted(currentEpochCounter uint64, header *flow.Header) {
	p.logger.Info().Uint64("currentEpochCounter", currentEpochCounter).
		Uint64("height", header.Height).
		Uint64("view", header.View).
		Msg("epoch committed phase started")
}
