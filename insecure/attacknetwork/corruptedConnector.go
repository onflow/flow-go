package attacknetwork

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
)

type CorruptedConnector struct {
	attackerAddress      string
	corruptedNodeIds     flow.IdentityList
	corruptedPortMapping map[flow.Identifier]string
}

func NewCorruptedConnector(corruptedNodeIds flow.IdentityList, corruptedPortMapping map[flow.Identifier]string) *CorruptedConnector {
	return &CorruptedConnector{
		corruptedNodeIds:     corruptedNodeIds,
		corruptedPortMapping: corruptedPortMapping,
	}
}

// Connect creates a connection the corruptible conduit factory of the given corrupted identity.
func (c *CorruptedConnector) Connect(ctx context.Context, targetId flow.Identifier) (insecure.CorruptedNodeConnection, error) {
	if len(c.attackerAddress) == 0 {
		return nil, fmt.Errorf("attacker address has not set on the connector")
	}

	port, ok := c.corruptedPortMapping[targetId]
	if !ok {
		return nil, fmt.Errorf("could not find port mapping for corrupted id: %x", targetId)
	}

	corruptedAddress := fmt.Sprintf("localhost:%s", port)
	gRpcClient, err := grpc.Dial(
		corruptedAddress,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("could not dial corruptible conduit factory %s: %w", corruptedAddress, err)
	}

	client := insecure.NewCorruptibleConduitFactoryClient(gRpcClient)

	_, err = client.RegisterAttacker(ctx, &insecure.AttackerRegisterMessage{
		Address: c.attackerAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("could not register attacker: %w", err)
	}

	stream, err := client.ProcessAttackerMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not establish a stream to corruptible conduit factory: %w", err)
	}

	return &CorruptedNodeConnection{stream: stream}, nil
}

func (c *CorruptedConnector) WithAttackerAddress(address string) {
	c.attackerAddress = address
}
