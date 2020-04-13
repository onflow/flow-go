package ghost

import (
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/integration/ghost/engine/ghost/rpc"
	"github.com/dapperlabs/flow-go/module"
)

func main() {

	var (
		rpcConf rpc.Config
	)

	cmd.FlowNode("access").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "r", "localhost:9000", "the address the GRPC server listens on")
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, node.State, rpcConf)
			return rpcEng, nil
		}).
		Run()
}
