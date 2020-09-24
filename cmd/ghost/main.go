package main

import (
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/engine/ghost/engine"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/gossip/libp2p/validators"
)

func main() {
	var (
		rpcConf engine.Config
	)

	cmd.FlowNode("ghost").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "r", "localhost:9000", "the address the GRPC server listens on")
		}).
		Module("message validators", func(node *cmd.FlowNodeBuilder) error {
			node.MsgValidators = []validators.MessageValidator{
				// filter out messages sent by this node itself
				validators.NewSenderValidator(node.Me.NodeID()),
				// but retain all the 1-k messages even if they are not intended for this node
			}
			return nil
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng, err := engine.New(node.Network, node.Logger, node.Me, rpcConf)
			return rpcEng, err
		}).
		Run()
}
