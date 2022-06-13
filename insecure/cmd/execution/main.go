package main

import (
	"fmt"

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
	fmt.Println("insecure/cmd/execution/main.go>6")
	corruptedExecutionBuilder.LoadComponentsAndModules()
	fmt.Println("insecure/cmd/execution/main.go>7")
	node, err := corruptedExecutionBuilder.FlowNodeBuilder.Build()
	fmt.Println("insecure/cmd/execution/main.go>8")
	if err != nil {
		fmt.Println("insecure/cmd/execution/main.go>9")
		corruptedExecutionBuilder.FlowNodeBuilder.Logger.Fatal().Err(err).Send()
		fmt.Println("insecure/cmd/execution/main.go>9.1")
	}
	fmt.Println("insecure/cmd/execution/main.go>10")
	node.Run()
	fmt.Println("insecure/cmd/execution/main.go>11")
}
