package insecure

import (
	"context"
)

type CorruptedNodeConnection interface {
	SendMessage(*Message) error
	CloseConnection() error
}

type CorruptedNodeConnector interface {
	// Connect creates a gRPC client for the corruptible conduit factory of the given corrupted identity. It then
	// connects the client to the remote corruptible conduit factory and returns it.
	Connect(context.Context, string) (CorruptedNodeConnection, error)
}
