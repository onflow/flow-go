package main

import (
	"fmt"
	"github.com/onflow/flow-go/cmd"
	insecmd "github.com/onflow/flow-go/insecure/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	fmt.Println("insecure/cmd/execution/main.go>1")
	corruptedBuilder := insecmd.NewCorruptedNodeBuilder(flow.RoleExecution.String())
	fmt.Println("insecure/cmd/execution/main.go>2")
	corruptedExecutionBuilder := cmd.NewExecutionNodeBuilder(corruptedBuilder.FlowNodeBuilder)
	fmt.Println("insecure/cmd/execution/main.go>3")
	corruptedExecutionBuilder.LoadFlags()
	fmt.Println("insecure/cmd/execution/main.go>4")

	if err := corruptedBuilder.Initialize(); err != nil {
		fmt.Println("insecure/cmd/execution/main.go>5")
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
	}
	fmt.Println("insecure/cmd/execution/main.go>10")
	node.Run()
	fmt.Println("insecure/cmd/execution/main.go>11")
}
