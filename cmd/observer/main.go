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

	anb.WithNewObserverServiceBuilder()

	if err := anb.Initialize(); err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}

	node, err := anb.Build()
	if err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
