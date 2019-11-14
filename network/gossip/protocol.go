package gossip

import "github.com/dapperlabs/flow-go/proto/gossip/messages"

type ProtocolMessage struct {
	Message   messages.GossipMessage
	Broadcast bool
}

type Protocol interface {
	Handle(onReceive func(sender string, msg ProtocolMessage)) error
	Start(address string, port string) error
	Stop() error

	Dial(address string) (Connection, error)
}

type Connection interface {
	Send(msg *ProtocolMessage) error
	Close() error
}
