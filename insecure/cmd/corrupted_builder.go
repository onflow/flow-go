package cmd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/insecure/corruptible"
	"github.com/onflow/flow-go/module"
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

	cnb.enqueueCorruptibleConduitFactory() // initializes corrupted conduit factory (ccf).

	return nil
}

func (cnb *CorruptedNodeBuilder) enqueueCorruptibleConduitFactory() {
	cnb.FlowNodeBuilder.OverrideComponent(cmd.ConduitFactoryComponent, func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		host, _, err := net.SplitHostPort(node.BindAddr)
		if err != nil {
			return nil, fmt.Errorf("could not extract host address: %w", err)
		}

		address := net.JoinHostPort(host, strconv.Itoa(CorruptibleConduitFactoryPort))
		ccf := corruptible.NewCorruptibleConduitFactory(
			cnb.FlowNodeBuilder.Logger,
			cnb.FlowNodeBuilder.RootChainID,
			cnb.FlowNodeBuilder.NodeID,
			cnb.FlowNodeBuilder.CodecFactory(), address)

		cnb.FlowNodeBuilder.ConduitFactory = ccf
		return ccf, nil
	})

	cnb.FlowNodeBuilder.OverrideComponent(cmd.NetworkComponent, func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return cnb.FlowNodeBuilder.InitFlowNetworkWithConduitFactory(node, cnb.FlowNodeBuilder.ConduitFactory)
	})
}
