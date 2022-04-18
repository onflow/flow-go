package main

import (
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	// TODO: use corrupted verification builder when available.
	verificationBuilder := cmd.NewVerificationNodeBuilder(cmd.FlowNode(flow.RoleVerification.String()))
	verificationBuilder.LoadFlags()

	if err := verificationBuilder.FlowNodeBuilder.Initialize(); err != nil {
		verificationBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}

	verificationBuilder.LoadComponentsAndModules()

	node, err := verificationBuilder.FlowNodeBuilder.Build()
	if err != nil {
		verificationBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
