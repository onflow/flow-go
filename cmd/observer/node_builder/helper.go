package node_builder

import "github.com/onflow/flow-go/cmd/access/node_builder"

func RunObserverNode() {
	// Use the generic Access Node builder as a temporary solution.
	// An Observer service is currently implemented as an unstaked Access node
	onb := node_builder.FlowAccessNode()

	onb.PrintBuildVersionDetails()

	// parse all the command line args
	if err := onb.ParseFlags(); err != nil {
		onb.Logger.Fatal().Err(err).Send()
	}

	// choose a staked or an unstaked node builder based on anb.staked
	var builder node_builder.AccessNodeBuilder
	if onb.IsStaked() {
		builder = node_builder.NewStakedAccessNodeBuilder(onb)
	} else {
		builder = node_builder.NewUnstakedAccessNodeBuilder(onb)
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
