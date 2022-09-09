package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

type CorruptMessageProcessor interface {
	network.MessageProcessor
	RelayToOriginalProcessor(channel channels.Channel, originID flow.Identifier, event interface{}) error
}
