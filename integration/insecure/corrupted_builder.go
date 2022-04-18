package insecure

import (
	"fmt"
	"net"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/insecure/corruptible"
	"github.com/onflow/flow-go/module"
)

// CorruptibleConduitFactoryPort is the port number that gRPC server of the conduit factory of corrupted nodes is listening on.
const CorruptibleConduitFactoryPort = "4300"

// CorruptedNodeBuilder corrupted node builder creates a general flow node builder with corruptible conduit factory.
type CorruptedNodeBuilder struct {
	*cmd.FlowNodeBuilder
	ccf *corruptible.ConduitFactory
}

func (cnb *CorruptedNodeBuilder) Initialize() error {
	if err := cnb.FlowNodeBuilder.Initialize(); err != nil {
		return fmt.Errorf("could not initilized flow node builder: %w", err)
	}

	cnb.enqueueCorruptibleConduitFactory() // initializes corrupted conduit factory (ccf).
	cnb.overrideCorruptedNetwork()         // initializes network with ccf.

	return nil
}

func (cnb *CorruptedNodeBuilder) enqueueCorruptibleConduitFactory() {
	cnb.Component("corruptible-conduit-factory", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		host, _, err := net.SplitHostPort(node.BindAddr)
		if err != nil {
			return nil, fmt.Errorf("could not extract host address: %w", err)
		}

		address := net.JoinHostPort(host, CorruptibleConduitFactoryPort)
		ccf := corruptible.NewCorruptibleConduitFactory(cnb.Logger, cnb.RootChainID, cnb.NodeID, cnb.CodecFactory(), address)

		cnb.ccf = ccf
		return ccf, nil
	})
}

func (cnb *CorruptedNodeBuilder) overrideCorruptedNetwork() {
	cnb.OverrideComponent("network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return cnb.InitFlowNetworkWithConduitFactory(node, cnb.ccf)
	})
}
