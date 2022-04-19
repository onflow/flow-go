package main

import (
	"github.com/onflow/flow-go/cmd"
	insecmd "github.com/onflow/flow-go/insecure/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	//verificationBuilder := cmd.NewVerificationNodeBuilder(
	//	cmd.FlowNode(flow.RoleVerification.String()))
	verificationBuilder := cmd.NewVerificationNodeBuilder(
		insecmd.NewCorruptedNodeBuilder(flow.RoleVerification.String()).FlowNodeBuilder)
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
