package corruptible

import (
	"github.com/onflow/flow-go/model/flow"
	flownet "github.com/onflow/flow-go/network"
)

type MessageProcessor struct {
	originalProcessor flownet.MessageProcessor // original message processor
}

var _ flownet.MessageProcessor = &MessageProcessor{}

func (m *MessageProcessor) Process(channel flownet.Channel, originID flow.Identifier, message interface{}) error {
	// TODO: instead of passing through the ingress traffic, process should
	// relay it to the attacker.
	return m.originalProcessor.Process(channel, originID, message)
}
