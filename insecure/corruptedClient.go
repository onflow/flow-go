package insecure

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// CorruptedNodeConnection abstracts connection from orchestrator to a corrupted conduit factory through the attack network.
type CorruptedNodeConnection interface {
	// SendMessage sends the message from orchestrator to the corrupted conduit factory.
	SendMessage(*Message) error

	// CloseConnection closes the connection to the corrupted conduit factory.
	CloseConnection() error
}

// CorruptedNodeConnector establishes a connection to a remote corrupted node.
type CorruptedNodeConnector interface {
	// Connect creates a connection the corruptible conduit factory of the given corrupted identity.
	Connect(context.Context, flow.Identifier) (CorruptedNodeConnection, error)

	// WithAttackerAddress sets the address of attacker for registration on corruptible conduit factories.
	WithAttackerAddress(string)
}
