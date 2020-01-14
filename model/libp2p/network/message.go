package network

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// NetworkMessage represents the message data structure of an application layer event
type NetworkMessage struct {
	ChannelID uint8
	EventID   []byte
	OriginID  flow.Identifier
	TargetIDs []flow.Identifier
	Payload   []byte
}
