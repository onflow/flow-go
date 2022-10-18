package main

import (
	"github.com/onflow/flow-go/cmd"
	insecmd "github.com/onflow/flow-go/insecure/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	corruptBuilder := insecmd.NewCorruptNodeBuilder(flow.RoleExecution.String())
	corruptExecutionBuilder := cmd.NewExecutionNodeBuilder(corruptBuilder.FlowNodeBuilder)
	corruptExecutionBuilder.LoadFlags()

	if err := corruptBuilder.Initialize(); err != nil {
		corruptExecutionBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}

	corruptExecutionBuilder.LoadComponentsAndModules()

	node, err := corruptExecutionBuilder.FlowNodeBuilder.Build()
	if err != nil {
		corruptExecutionBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
