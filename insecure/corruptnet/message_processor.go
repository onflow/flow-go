package corruptnet

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

var _ flownet.MessageProcessor = &MessageProcessor{}

func NewCorruptMessageProcessor(logger zerolog.Logger, originalProcessor flownet.MessageProcessor, ingressController insecure.IngressController) *MessageProcessor {
	return &MessageProcessor{
		logger:            logger,
		originalProcessor: originalProcessor,
		ingressController: ingressController,
	}
}

// Process implements handling ingress (incoming) messages from the honest Flow network (via network.MessageProcessor interface).
// If an Attacker is registered on the Corrupt Network, then these ingress messages are passed to the Attacker (by the Corrupt Network).
// If an Attacker is not registered on the Corrupt Network, then these ingress messages are passed to the original (honest) Message Processor.
func (m *MessageProcessor) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	// Relay message to the attack orchestrator.
	lg := m.logger.With().
		Str("channel", string(channel)).
		Str("origin_id", fmt.Sprintf("%v", originID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event)).Logger()
	lg.Debug().Msg("processing new incoming event")
	attackerRegistered := m.ingressController.HandleIncomingEvent(event, channel, originID)
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
