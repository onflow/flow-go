package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/module"
)

func main() {

	cmd.FlowNode("execution").
		CreateReadDoneAware("execution engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing execution engine")

			exec, err := execution.New(node.Logger, node.Network, node.Me)
			node.MustNot(err).Msg("could not initialize execution engine")
			return exec
		}).Run()

}
