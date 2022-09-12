package corruptible

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

type MessageProcessor struct {
	logger            zerolog.Logger
	ingressController insecure.IngressController
	originalProcessor flownet.MessageProcessor // original message processor
}

var _ insecure.CorruptMessageProcessor = &MessageProcessor{}

func NewCorruptMessageProcessor(logger zerolog.Logger, originalProcessor flownet.MessageProcessor, ingressController insecure.IngressController) *MessageProcessor {
	return &MessageProcessor{
		logger:            logger,
		originalProcessor: originalProcessor,
		ingressController: ingressController,
	}
}

// RelayToOriginalProcessor relays the message to the original message processor.
func (m *MessageProcessor) RelayToOriginalProcessor(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	return m.originalProcessor.Process(channel, originID, event)
}

func (m *MessageProcessor) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	// Relay message to the attack orchestrator.
	lg := m.logger.With().
		Str("channel", string(channel)).
		Str("origin_id", fmt.Sprintf("%v", originID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event)).Logger()
	lg.Debug().Msg("processing new incoming event")
	attackerRegistered := m.ingressController.HandleIncomingEvent(channel, originID, event)
	if !attackerRegistered {
		// No attack orchestrator registered yet, hence pass the ingress message back to the original processor.
		err := m.originalProcessor.Process(channel, originID, event)
		if err != nil {
			m.logger.Fatal().Err(err).Msg("could not send message back to original message processor")
			return err
		}
		lg.Debug().Msg("incoming event processed by original processor as no attacker registered")
	} else {
		lg.Debug().Msg("incoming event processed by the registered attacker")
	}
	return nil
}
