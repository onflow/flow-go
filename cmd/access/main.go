package main

import (
	nodebuilder "github.com/onflow/flow-go/cmd/access/node_builder"
)

func main() {
	anb := nodebuilder.FlowAccessNode() // use the generic Access Node builder till it is determined if this is a staked AN or an unstaked AN

	anb.PrintBuildVersionDetails()

	// parse all the command line args
	anb.ParseFlags()

	// choose a staked or an unstaked node builder based on anb.staked
	var nodeBuilder nodebuilder.AccessNodeBuilder
	if anb.IsStaked() {
		nodeBuilder = nodebuilder.NewStakedAccessNodeBuilder(anb)
	} else {
		nodeBuilder = nodebuilder.NewUnstakedAccessNodeBuilder(anb)
	}

	if err := nodeBuilder.Initialize(); err != nil {
		anb.Logger.Fatal().Err(err).Send()
	}

	nodeBuilder.
		SerialStart().
		Build().
		Run(nodeBuilder.PostShutdown)
}
