package main

import (
	"github.com/onflow/flow-go/cmd"
	insecmd "github.com/onflow/flow-go/insecure/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	corruptBuilder := insecmd.NewCorruptNodeBuilder(flow.RoleVerification.String())
	corruptVerificationBuilder := cmd.NewVerificationNodeBuilder(corruptBuilder.FlowNodeBuilder)
	corruptVerificationBuilder.LoadFlags()

	if err := corruptBuilder.Initialize(); err != nil {
		corruptVerificationBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}

	corruptVerificationBuilder.LoadComponentsAndModules()

	node, err := corruptVerificationBuilder.FlowNodeBuilder.Build()
	if err != nil {
		corruptVerificationBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
