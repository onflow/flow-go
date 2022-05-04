package main

import (
	nodebuilder "github.com/onflow/flow-go/cmd/access/node_builder"
)

func main() {
	anb := nodebuilder.FlowAccessNode()

	anb.PrintBuildVersionDetails()

	// parse all the command line args
	if err := anb.ParseFlags(); err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}

	var builder nodebuilder.AccessNodeBuilder = nodebuilder.NewUnstakedAccessNodeBuilder(anb)

	if err := builder.Initialize(); err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}

	node, err := builder.Build()
	if err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
