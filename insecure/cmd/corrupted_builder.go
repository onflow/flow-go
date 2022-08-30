package cmd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/insecure/corruptible"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/logging"
)

// CorruptibleConduitFactoryPort is the port number that gRPC server of the conduit factory of corrupted nodes is listening on.
const CorruptibleConduitFactoryPort = 4300

// CorruptedNodeBuilder creates a general flow node builder with corruptible conduit factory.
type CorruptedNodeBuilder struct {
	*cmd.FlowNodeBuilder
}

func NewCorruptedNodeBuilder(role string) *CorruptedNodeBuilder {
	return &CorruptedNodeBuilder{
		FlowNodeBuilder: cmd.FlowNode(role),
	}
}

func (cnb *CorruptedNodeBuilder) Initialize() error {
	if err := cnb.FlowNodeBuilder.Initialize(); err != nil {
		return fmt.Errorf("could not initilized flow node builder: %w", err)
	}

	cnb.enqueueNetworkingLayer() // initializes corrupted networking layer.

	return nil
}

func (cnb *CorruptedNodeBuilder) enqueueNetworkingLayer() {
	cnb.FlowNodeBuilder.OverrideComponent(cmd.NetworkComponent, func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		myAddr := cnb.FlowNodeBuilder.NodeConfig.Me.Address()
		if cnb.FlowNodeBuilder.BaseConfig.BindAddr != cmd.NotSet {
			myAddr = cnb.FlowNodeBuilder.BaseConfig.BindAddr
		}

		host, _, err := net.SplitHostPort(myAddr)
		if err != nil {
			return nil, fmt.Errorf("could not extract host address: %w", err)
		}

		address := net.JoinHostPort(host, strconv.Itoa(CorruptibleConduitFactoryPort))
		ccf := corruptible.NewCorruptibleConduitFactory(cnb.FlowNodeBuilder.Logger, cnb.FlowNodeBuilder.RootChainID)

		cnb.Logger.Info().Hex("node_id", logging.ID(cnb.NodeID)).Msg("corrupted conduit factory initiated")

		flowNetwork, err := cnb.FlowNodeBuilder.InitFlowNetworkWithConduitFactory(node, ccf)
		if err != nil {
			return nil, fmt.Errorf("could not initiate flow network: %w", err)
		}

		// initializes corruptible network that acts as a wrapper around the original flow network of the node, hence
		// allowing a remote attacker to control the ingress and egress traffic of the node.
		corruptibleNetwork, err := corruptible.NewCorruptNetwork(
			cnb.Logger,
			cnb.RootChainID,
			address,
			cnb.Me,
			cnb.CodecFactory(),
			flowNetwork,
			ccf)
		if err != nil {
			return nil, fmt.Errorf("could not create corruptible network: %w", err)
		}
		cnb.Logger.Info().Hex("node_id", logging.ID(cnb.NodeID)).Str("address", address).Msg("corruptible network initiated")
		return corruptibleNetwork, nil
	})
}
