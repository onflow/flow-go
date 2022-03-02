package adversary

import (
	"fmt"
	"net"
	"strconv"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

type AttackNetwork struct {
	corruptedNodes map[flow.Identifier]insecure.CorruptibleConduitFactoryClient
}

func NewAttackNetwork(corruptedIds flow.IdentityList) error {
	attackNetwork := &AttackNetwork{corruptedNodes: make(map[flow.Identifier]insecure.CorruptibleConduitFactoryClient)}

	for _, corruptedId := range corruptedIds {
		address, _, err := net.SplitHostPort(corruptedId.Address)
		if err != nil {
			return fmt.Errorf("could not extract address of corruptible conduit factory %s of node %x: %w", address, corruptedId, err)
		}

		corruptedFactoryAddress := net.JoinHostPort(address, strconv.Itoa(insecure.CorruptedFactoryPort))
		gRpcClient, err := grpc.Dial(corruptedFactoryAddress)
		if err != nil {
			return fmt.Errorf("could not dial corrupted conduit factory %s of node %x: %w", corruptedFactoryAddress, corruptedId, err)
		}
		corruptedNodeClient := insecure.NewCorruptibleConduitFactoryClient(gRpcClient)

		attackNetwork.corruptedNodes[corruptedId.NodeID] = corruptedNodeClient
	}

}

func (a AttackNetwork) RpcUnicastOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, targetId flow.Identifier) error {
	a.corruptedNodes[corruptedId].ProcessAttackerMessage()
}

func (a AttackNetwork) RpcPublishOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, targetIds ...flow.Identifier) error {
	panic("implement me")
}

func (a AttackNetwork) RpcMulticastOnChannel(corruptedId flow.Identifier, channel network.Channel, event interface{}, num uint, targetIds ...flow.Identifier) error {
	panic("implement me")
}

func

var _ insecure.AttackNetwork = &AttackNetwork{}
