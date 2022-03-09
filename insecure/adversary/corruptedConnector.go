package adversary

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
)

type CorruptedConnector struct {
	attackerAddress string
}

func (c *CorruptedConnector) Connect(ctx context.Context, address flow.Identifier) (insecure.CorruptedNodeConnection, error) {
	corruptedAddress, err := corruptedConduitFactoryAddress(address)
	if err != nil {
		return nil, fmt.Errorf("could not generate corruptible conduit factory address for: %w", err)
	}
	gRpcClient, err := grpc.Dial(corruptedAddress)
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

// corruptedConduitFactoryAddress generates and returns the gRPC interface address of corruptible conduit factory for given identity.
func corruptedConduitFactoryAddress(address string) (string, error) {
	corruptedAddress, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("could not extract address of corruptible conduit factory %s: %w", address, err)
	}

	return net.JoinHostPort(corruptedAddress, strconv.Itoa(insecure.CorruptedFactoryPort)), nil
}
