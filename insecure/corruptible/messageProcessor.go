package corruptible

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/channels"

	"github.com/onflow/flow-go/model/flow"
	flownet "github.com/onflow/flow-go/network"
)

type MessageProcessor struct {
	logger            zerolog.Logger
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
	// TODO: instead of passing through the ingress traffic, process should
	// relay it to the attacker.
	return m.originalProcessor.Process(channel, originID, message)
}
