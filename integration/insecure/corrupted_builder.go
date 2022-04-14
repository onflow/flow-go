package insecure

import (
	"fmt"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/insecure/corruptible"
	"github.com/onflow/flow-go/module"
)

type CorruptedNodeBuilder struct {
	*cmd.FlowNodeBuilder
	ccf *corruptible.ConduitFactory
}

func (cnb *CorruptedNodeBuilder) Initialize() error {
	if err := cnb.FlowNodeBuilder.Initialize(); err != nil {
		return fmt.Errorf("could not initilized flow node builder: %w", err)
	}
	cnb.overrideCorruptedNetwork()
}

func (cnb *CorruptedNodeBuilder) enqueueCorruptibleConduitFactory() {
	cnb.Component("corruptible-conduit-factory", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		// corruptible.NewCorruptibleConduitFactory(cnb.Logger, cnb.RootChainID, cnb.NodeID, , net.JoinHostPort("localhost", ))
	})
}

func (cnb *CorruptedNodeBuilder) overrideCorruptedNetwork() {
	cnb.OverrideComponent("network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return cnb.InitFlowNetworkWithConduitFactory(node, cnb.ccf)
	})
}
