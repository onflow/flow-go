package main

import (
	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	exeBuilder := cmd.NewExecutionNodeBuilder(cmd.FlowNode(flow.RoleExecution.String()))
	exeBuilder.LoadFlags()

	if err := exeBuilder.Initialize(); err != nil {
		exeBuilder.Logger.Fatal().Err(err).Send()
	}

	exeBuilder.LoadComponentsAndModules()

	node, err := exeBuilder.Build()
	if err != nil {
		exeBuilder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
