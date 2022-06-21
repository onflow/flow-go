package attacknetwork

import (
	"fmt"

	"github.com/onflow/flow-go/insecure"
)

// CorruptedNodeConnection abstracts connection from orchestrator to a corrupted conduit factory through the attack network.
type CorruptedNodeConnection struct {
	stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient
}

// SendMessage sends the message from orchestrator to the corrupted conduit factory.
func (c *CorruptedNodeConnection) SendMessage(message *insecure.Message) error {
	err := c.stream.Send(message)
	if err != nil {
		return fmt.Errorf("could not send message: %w", err)
	}

	return nil
}

// CloseConnection closes the connection to the corrupted conduit factory.
func (c *CorruptedNodeConnection) CloseConnection() error {
	err := c.stream.CloseSend()
	if err != nil {
		return fmt.Errorf("could not close grpc send connection: %w", err)
	}

	return nil
}
