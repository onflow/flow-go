package main

import (
	"github.com/onflow/flow-go/cmd"
	insecmd "github.com/onflow/flow-go/insecure/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	corruptedBuilder := insecmd.NewCorruptedNodeBuilder(flow.RoleVerification.String())
	corruptedVerificationBuilder := cmd.NewVerificationNodeBuilder(corruptedBuilder.FlowNodeBuilder)
	corruptedVerificationBuilder.LoadFlags()

	if err := corruptedBuilder.Initialize(); err != nil {
		corruptedVerificationBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}

	corruptedVerificationBuilder.LoadComponentsAndModules()

	node, err := corruptedVerificationBuilder.FlowNodeBuilder.Build()
	if err != nil {
		corruptedVerificationBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
