package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// CorruptedNodeConnection abstracts connection from attacker to a corrupt network through the orchestrator network.
type CorruptedNodeConnection interface {
	// SendMessage sends the message from orchestrator to the corrupted conduit factory.
	SendMessage(*Message) error

	// CloseConnection closes gRPC client connection to the corrupt network (gRPC server).
	CloseConnection() error
}

// CorruptedNodeConnector establishes a connection to a remote corrupted node.
type CorruptedNodeConnector interface {
	// Connect creates a connection the corruptible conduit factory of the given corrupted identity.
	Connect(irrecoverable.SignalerContext, flow.Identifier) (CorruptedNodeConnection, error)

	// WithIncomingMessageHandler sets the handler for the incoming messages from remote corrupted nodes.
	WithIncomingMessageHandler(func(*Message))
}
