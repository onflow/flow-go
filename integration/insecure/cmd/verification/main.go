package main

import (
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/integration/insecure"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	verificationBuilder := cmd.NewVerificationNodeBuilder(
		insecure.NewCorruptedNodeBuilder(flow.RoleExecution.String()).FlowNodeBuilder)
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
