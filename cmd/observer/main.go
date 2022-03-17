package main

import (
	observer "github.com/onflow/flow-go/cmd/observer/node_builder"
)

func main() {
	onb := observer.FlowAccessNode()

	onb.PrintBuildVersionDetails()

	// parse all the command line args
	if err := onb.ParseFlags(); err != nil {
		onb.Logger.Fatal().Err(err).Send()
	}

	// create an observer builder
	var builder observer.ObserverServiceBuilder = observer.NewObserverNodeBuilder(onb)
	if builder == nil {
		return
	}

	if err := builder.Initialize(); err != nil {
		onb.Logger.Fatal().Err(err).Send()
	}

	node, err := builder.Build()
	if err != nil {
		onb.Logger.Fatal().Err(err).Send()
	}
	node.Run()
}
