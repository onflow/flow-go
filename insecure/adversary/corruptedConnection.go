package adversary

import (
	"fmt"

	"github.com/onflow/flow-go/insecure"
)

type CorruptedNodeConnection struct {
	stream insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient
}

func (c *CorruptedNodeConnection) SendMessage(message *insecure.Message) error {
	err := c.stream.Send(message)
	if err != nil {
		return fmt.Errorf("could not send message: %w", err)
	}

	return nil
}

func (c *CorruptedNodeConnection) CloseConnection() error {
	err := c.stream.CloseSend()
	if err != nil {
		return fmt.Errorf("could not close grpc send connection: %w", err)
	}

	return nil
}
