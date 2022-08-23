package corruptible

import (
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

func NewCorruptMessageProcessor(logger zerolog.Logger, originalProcessor flownet.MessageProcessor) *MessageProcessor {
	return &MessageProcessor{
		logger:            logger,
		originalProcessor: originalProcessor,
	}
}

func (m *MessageProcessor) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	// Relay message to the attack orchestrator.
	attackerRegistered := m.ingressController.HandleIncomingEvent(channel, originID, message)
	if !attackerRegistered {
		// No attack orchestrator registered yet, hence pass the ingress message back to the original processor.
		err := m.originalProcessor.Process(channel, originID, message)
		if err != nil {
			m.logger.Fatal().Err(err).Msg("could not send message back to original message processor")
			return err
		}
	}
	return nil
}
