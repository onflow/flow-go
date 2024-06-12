package pubsub

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/utils/logging"
)

// LogConsumer implements protocol_state.StateMachineEventsConsumer and logs all events.
type LogConsumer struct {
	log zerolog.Logger
}

var _ protocol_state.StateMachineEventsConsumer = (*LogConsumer)(nil)

func NewLogConsumer(log zerolog.Logger) *LogConsumer {
	lc := &LogConsumer{
		log: log,
	}
	return lc
}

func (l *LogConsumer) OnInvalidServiceEvent(event flow.ServiceEvent, err error) {
	l.log.Warn().
		Str(logging.KeySuspicious, "true").
		Str("type", event.Type.String()).
		Msgf("invalid service event detected: %s", err.Error())
}

func (l *LogConsumer) OnServiceEventReceived(event flow.ServiceEvent) {
	l.log.Info().
		Str("type", event.Type.String()).
		Msg("received service event")
}

func (l *LogConsumer) OnServiceEventProcessed(event flow.ServiceEvent) {
	l.log.Info().
		Str("type", event.Type.String()).
		Msg("successfully processed service event")
}
