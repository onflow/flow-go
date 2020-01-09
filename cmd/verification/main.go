package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/module"
)

func main() {

	cmd.FlowNode("verification").
		Create(func(node *cmd.FlowNodeBuilder) {

		}).
		Component("verifier engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing verifier engine")

			vrfy, err := verifier.New(node.Logger, node.Network, node.State, node.Me)
			node.MustNot(err).Msg("could not initialize verifier engine")
			return vrfy
		}).
		Run()
}
