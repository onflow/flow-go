package main

import (
	nodebuilder "github.com/onflow/flow-go/cmd/access/node_builder"
)

func main() {
	builder := nodebuilder.FlowAccessNode() // use the generic Access Node builder till it is determined if this is a staked AN or an unstaked AN

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
