package main

import (
	nodebuilder "github.com/onflow/flow-go/cmd/access/node_builder"
)

func main() {
	anb := nodebuilder.FlowAccessNode() // use the generic Access Node builder till it is determined if this is a staked AN or an unstaked AN

	anb.PrintBuildVersionDetails()

	// parse all the command line args
	if err := anb.ParseFlags(); err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}

	// Please use an observer for unstaked workloads going forward
	var builder nodebuilder.AccessNodeBuilder = nodebuilder.NewStakedAccessNodeBuilder(anb)

	if err := builder.Initialize(); err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}

	node, err := builder.Build()
	if err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
