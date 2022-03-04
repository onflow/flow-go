package adversary

import (
	"fmt"
	"net"
	"strconv"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network"
)

type AttackNetwork struct {
	component.Component
	corruptedIds     flow.IdentityList
	corruptedClients map[flow.Identifier]insecure.CorruptibleConduitFactoryClient
}

func NewAttackNetwork(corruptedIds flow.IdentityList) *AttackNetwork {
	attackNetwork := &AttackNetwork{
		corruptedIds:     corruptedIds,
		corruptedClients: make(map[flow.Identifier]insecure.CorruptibleConduitFactoryClient),
	}

	return attackNetwork
}

func (a AttackNetwork) RpcUnicastOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, targetId flow.Identifier) error {
	a.corruptedClients[corruptedId].ProcessAttackerMessage()
}

func (a AttackNetwork) RpcPublishOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, targetIds ...flow.Identifier) error {
	panic("implement me")
}

func (a AttackNetwork) RpcMulticastOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, num uint, targetIds ...flow.Identifier) error {
	panic("implement me")
}

func (a *AttackNetwork) start() error {
	for _, corruptedId := range a.corruptedIds {
		corruptibleClient, err := a.corruptibleConduitFactoryClient(corruptedId.Address)
		if err != nil {
			return fmt.Errorf("could not establish corruptible client to node %x: %w", corruptedId.NodeID, err)
		}
		a.corruptedClients[corruptedId.NodeID] = corruptibleClient
	}

	return nil
}

// corruptedConduitFactoryAddress generates and returns the gRPC interface address of corruptible conduit factory for given identity.
func corruptedConduitFactoryAddress(address string) (string, error) {
	corruptedAddress, _, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("could not extract address of corruptible conduit factory %s: %w", address, err)
	}

	return net.JoinHostPort(corruptedAddress, strconv.Itoa(insecure.CorruptedFactoryPort)), nil
}

// corruptibleConduitFactoryClient creates a gRPC client for the corruptible conduit factory of the given corrupted identity. It then
// connects the client to the remote corruptible conduit factory and returns it.+
func (a *AttackNetwork) corruptibleConduitFactoryClient(address string) (insecure.CorruptibleConduitFactoryClient, error) {
	corruptedAddress, err := corruptedConduitFactoryAddress(address)
	if err != nil {
		return nil, fmt.Errorf("could not generate corruptible conduit factory address for: %w", err)
	}
	gRpcClient, err := grpc.Dial(corruptedAddress)
	if err != nil {
		return nil, fmt.Errorf("could not dial corruptible conduit factory %s: %w", corruptedAddress, err)
	}

	return insecure.NewCorruptibleConduitFactoryClient(gRpcClient), nil
}
