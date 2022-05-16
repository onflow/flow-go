package main

import (
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	exeBuilder := cmd.NewExecutionNodeBuilder(cmd.FlowNode(flow.RoleExecution.String()))
	exeBuilder.LoadFlags()

	if err := exeBuilder.FlowNodeBuilder.Initialize(); err != nil {
		exeBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}

	exeBuilder.LoadComponentsAndModules()

	node, err := exeBuilder.FlowNodeBuilder.Build()
	if err != nil {
		exeBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
