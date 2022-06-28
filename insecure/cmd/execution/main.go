package main

import (
	"github.com/onflow/flow-go/cmd"
	insecmd "github.com/onflow/flow-go/insecure/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	corruptedBuilder := insecmd.NewCorruptedNodeBuilder(flow.RoleExecution.String())
	corruptedExecutionBuilder := cmd.NewExecutionNodeBuilder(corruptedBuilder.FlowNodeBuilder)
	corruptedExecutionBuilder.LoadFlags()

	if err := corruptedBuilder.Initialize(); err != nil {
		corruptedExecutionBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}

	corruptedExecutionBuilder.LoadComponentsAndModules()

	node, err := corruptedExecutionBuilder.FlowNodeBuilder.Build()
	if err != nil {
		corruptedExecutionBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
