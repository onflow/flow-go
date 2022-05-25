package attacknetwork

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
)

type CorruptedConnector struct {
	attackerAddress  string
	corruptedNodeIds flow.IdentityList
	ccfPort          int // conventional port for dialing remote corruptible conduit factories
}

func NewCorruptedConnector(corruptedNodeIds flow.IdentityList, ccfPort int) *CorruptedConnector {
	return &CorruptedConnector{
		corruptedNodeIds: corruptedNodeIds,
		ccfPort:          ccfPort,
	}
}

// Connect creates a connection the corruptible conduit factory of the given corrupted identity.
func (c *CorruptedConnector) Connect(ctx context.Context, targetId flow.Identifier) (insecure.CorruptedNodeConnection, error) {
	if len(c.attackerAddress) == 0 {
		return nil, fmt.Errorf("attacker address has not set on the connector")
	}

	corruptedAddress, err := c.corruptedConduitFactoryAddress(targetId)
	if err != nil {
		return nil, fmt.Errorf("could not generate corruptible conduit factory address for: %w", err)
	}
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

// corruptedConduitFactoryAddress generates and returns the gRPC interface address of corruptible conduit factory for given identity.
func (c *CorruptedConnector) corruptedConduitFactoryAddress(id flow.Identifier) (string, error) {
	identity, found := c.corruptedNodeIds.ByNodeID(id)
	if !found {
		return "", fmt.Errorf("could not find corrupted id for identifier: %x", id)
	}

	corruptedAddress, _, err := net.SplitHostPort(identity.Address)
	if err != nil {
		return "", fmt.Errorf("could not extract address of corruptible conduit factory %s: %w", identity.Address, err)
	}

	return net.JoinHostPort(corruptedAddress, strconv.Itoa(c.ccfPort)), nil
}
