package main

import (
	"github.com/onflow/flow-go/cmd"
	insecmd "github.com/onflow/flow-go/insecure/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	exeBuilder := cmd.NewExecutionNodeBuilder(insecmd.NewCorruptedNodeBuilder(flow.RoleExecution.String()).FlowNodeBuilder)
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
