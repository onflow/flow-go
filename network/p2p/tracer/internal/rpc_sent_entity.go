package internal

import (
	"github.com/onflow/flow-go/model/flow"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// rpcSentEntity struct representing an RPC control message sent from local node.
// This struct implements the flow.Entity interface and uses  messageID field deduplication.
type rpcSentEntity struct {
	// messageID the messageID of the rpc control message.
	messageID flow.Identifier
	// controlMsgType the control message type.
	controlMsgType p2pmsg.ControlMessageType
}

var _ flow.Entity = (*rpcSentEntity)(nil)

// ID returns the node ID of the sender, which is used as the unique identifier of the entity for maintenance and
// deduplication purposes in the cache.
func (r rpcSentEntity) ID() flow.Identifier {
	return r.messageID
}

// Checksum returns the node ID of the sender, it does not have any purpose in the cache.
// It is implemented to satisfy the flow.Entity interface.
func (r rpcSentEntity) Checksum() flow.Identifier {
	return r.messageID
}

// newRPCSentEntity returns a new rpcSentEntity.
func newRPCSentEntity(id flow.Identifier, controlMessageType p2pmsg.ControlMessageType) rpcSentEntity {
	return rpcSentEntity{
		messageID:      id,
		controlMsgType: controlMessageType,
	}
}
