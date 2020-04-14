package main

import (
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/ghost/engine"
	"github.com/dapperlabs/flow-go/module"
)

func main() {
	var (
		rpcConf engine.Config
	)

	cmd.FlowNode("ghost").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "r", "localhost:9000", "the address the GRPC server listens on")
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng, err := engine.New(node.Network, node.Logger, node.Me, rpcConf)
			return rpcEng, err
		}).
		Run()
}
