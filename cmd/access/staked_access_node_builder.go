package main

import (
	"github.com/onflow/flow-go/cmd"
)

type StakedAccessNodeBuilder struct {
	*FlowAccessNodeBuilder
}

func StakedAccessNode(anb *FlowAccessNodeBuilder) *StakedAccessNodeBuilder {
	return &StakedAccessNodeBuilder{
		FlowAccessNodeBuilder: anb,
	}
}

func (sanb *StakedAccessNodeBuilder) Initialize() cmd.NodeBuilder {

	// for the staked access node, initialize the network used to communicate with the other staked flow nodes
	sanb.EnqueueNetworkInit()

	// if an unstaked bind address is provided, initialize the network to communicate on the unstaked network
	if sanb.unstakedNetworkBindAddr != cmd.NotSet {
		sanb.EnqueueUnstakedNetworkInit()
	}

	sanb.EnqueueMetricsServerInit()

	sanb.RegisterBadgerMetrics()

	sanb.EnqueueTracer()

	return sanb
}
