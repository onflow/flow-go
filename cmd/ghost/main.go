package main

import (
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/engine/ghost/engine"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/validator"
	"github.com/onflow/flow-go/utils/grpcutils"
)

func main() {
	var (
		rpcConf engine.Config
	)

	nodeBuilder := cmd.FlowNode("ghost")
	nodeBuilder.ExtraFlags(func(flags *pflag.FlagSet) {
		flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "r", "localhost:9000", "the address the GRPC server listens on")
		flags.UintVar(&rpcConf.MaxMsgSize, "rpc-max-message-size", grpcutils.DefaultMaxMsgSize, "the maximum message size in bytes for messages sent or received over grpc")
	})

	if err := nodeBuilder.Initialize(); err != nil {
		nodeBuilder.Logger.Fatal().Err(err).Send()
	}

	nodeBuilder.
		Module("message validators", func(node *cmd.NodeConfig) error {
			validators := []network.MessageValidator{
				// filter out messages sent by this node itself
				validator.ValidateNotSender(node.Me.NodeID()),
				// but retain all the 1-k messages even if they are not intended for this node
			}
			node.MsgValidators = validators
			return nil
		}).
		Component("RPC engine", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			rpcEng, err := engine.New(node.EngineRegistry, node.Logger, node.Me, node.State, rpcConf)
			return rpcEng, err
		})

	node, err := nodeBuilder.Build()
	if err != nil {
		nodeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
