package main

import (
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	resend_engine "github.com/dapperlabs/flow-go/cmd/util/cmd/send-blocks/engine"
	"github.com/dapperlabs/flow-go/module"
)

func main() {

	var (
		flagHeightStart uint64
		flagHeightEnd   uint64
		flagDelayMs     uint16
	)

	cmd.FlowNode("block-sender").
		ExtraFlags(func(flags *pflag.FlagSet) {

			flags.Uint64Var(&flagHeightStart, "start-height", 1,
				"start block height")

			flags.Uint64Var(&flagHeightEnd, "end-height", 0,
				"end block height")

			flags.Uint16Var(&flagDelayMs, "delay", 10,
				"delay between block")

			//homedir, _ := os.UserHomeDir()
			//datadir := filepath.Join(homedir, ".flow", "execution")

			//flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			//flags.StringVar(&triedir, "triedir", datadir, "directory to store the execution State")
			//flags.Uint32Var(&mTrieCacheSize, "mtrie-cache-size", 1000, "cache size for MTrie")
			//flags.UintVar(&checkpointDistance, "checkpoint-distance", 1, "number of WAL segments between checkpoints")
		}).
		Component("resend engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			resendEngine := resend_engine.New(
				node.Logger,
				node.Network,
				node.Me,
				node.Storage,
				flagHeightStart,
				flagHeightEnd,
				flagDelayMs)

			return resendEngine, nil
		}).Run()
}
