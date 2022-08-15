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

func NewCorruptibleMessageProcessor(logger zerolog.Logger, originalProcessor flownet.MessageProcessor) *MessageProcessor {
	return &MessageProcessor{
		logger:            logger,
		originalProcessor: originalProcessor,
	}
}

func (m *MessageProcessor) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	// relay message to the attacker
	attackerRegistered := m.ingressController.HandleIncomingEvent(channel, originID, message)
	if !attackerRegistered {
		// if no attacker registered, pass it back to flow network and treat the message as a pass through
		err := m.originalProcessor.Process(channel, originID, message)
		if err != nil {
			m.logger.Fatal().Msgf("could not send message back to original message processor: %s", err)
			return err
		}
	}
	return nil
}
