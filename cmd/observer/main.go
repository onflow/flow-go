package main

import (
	nodebuilder "github.com/onflow/flow-go/cmd/observer/node_builder"
)

func main() {
	anb := nodebuilder.NewFlowObserverServiceBuilder()

	anb.PrintBuildVersionDetails()

	// parse all the command line args
	if err := anb.ParseFlags(); err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}

	var builder nodebuilder.ObserverBuilder = nodebuilder.NewObserverServiceBuilder(anb)

	if err := builder.Initialize(); err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}

	node, err := builder.Build()
	if err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
