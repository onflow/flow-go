package main

import (
	nodebuilder "github.com/onflow/flow-go/cmd/access/node_builder"
	insecmd "github.com/onflow/flow-go/insecure/cmd"
	"github.com/onflow/flow-go/model/flow"
)

func main() {
	corruptedBuilder := insecmd.NewCorruptedNodeBuilder(flow.RoleAccess.String())
	builder := nodebuilder.FlowAccessNode() // use the Access Node builder
	builder.FlowNodeBuilder = corruptedBuilder.FlowNodeBuilder
	builder.PrintBuildVersionDetails()

	// parse all the command line args
	if err := builder.ParseFlags(); err != nil {
		builder.Logger.Fatal().Err(err).Send()
	}

	if err := builder.Initialize(); err != nil {
		builder.Logger.Fatal().Err(err).Send()
	}

	node, err := builder.Build()
	if err != nil {
		builder.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
