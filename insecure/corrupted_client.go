package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// CorruptedNodeConnection abstracts connection from orchestrator to a corrupted conduit factory through the orchestrator network.
type CorruptedNodeConnection interface {
	// SendMessage sends the message from orchestrator to the corrupted conduit factory.
	SendMessage(*Message) error

	// CloseConnection closes the connection to the corrupted conduit factory.
	CloseConnection() error
}

// CorruptedNodeConnector establishes a connection to a remote corrupted node.
type CorruptedNodeConnector interface {
	// Connect creates a connection the corruptible conduit factory of the given corrupted identity.
	Connect(irrecoverable.SignalerContext, flow.Identifier) (CorruptedNodeConnection, error)

	// WithIncomingMessageHandler sets the handler for the incoming messages from remote corrupted nodes.
	WithIncomingMessageHandler(func(*Message))
}
