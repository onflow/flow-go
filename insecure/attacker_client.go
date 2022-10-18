package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// CorruptNodeConnection abstracts connection from attacker to a corrupt network through the attacker network.
type CorruptNodeConnection interface {
	// SendMessage sends the message from attacker to the corrupt network.
	SendMessage(*Message) error

	// CloseConnection closes gRPC client connection to the corrupt network (gRPC server).
	CloseConnection() error
}

// CorruptNodeConnector establishes a connection to a remote corrupt node.
type CorruptNodeConnector interface {
	// Connect creates a connection to the corrupt network of the given corrupt identity.
	Connect(irrecoverable.SignalerContext, flow.Identifier) (CorruptNodeConnection, error)

	// WithIncomingMessageHandler sets the handler for the incoming messages from remote corrupt nodes.
	WithIncomingMessageHandler(func(*Message))
}
