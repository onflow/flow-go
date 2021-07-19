package main

import (
	"time"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
)

// StakedAccessNodeBuilder builds a staked access node. The staked access node can optionally participate in the
// unstaked network publishing data for the unstaked access node downstream.
type StakedAccessNodeBuilder struct {
	*FlowAccessNodeBuilder
}

func StakedAccessNode(anb *FlowAccessNodeBuilder) *StakedAccessNodeBuilder {
	return &StakedAccessNodeBuilder{
		FlowAccessNodeBuilder: anb,
	}
}

// SupportUnstakedNodes returns True if this a staked Access Node which also participates in the unstaked network,
// False otherwise
func (sanb *StakedAccessNodeBuilder) supportUnstakedNodes() bool {
	// if an unstaked network bind address is provided, then this staked access node will act as the upstream for
	// unstaked access nodes
	return sanb.FlowAccessNodeBuilder.unstakedNetworkBindAddr != cmd.NotSet
}

func (sanb *StakedAccessNodeBuilder) Initialize() cmd.NodeBuilder {

	// for the staked access node, initialize the network used to communicate with the other staked flow nodes
	// by calling the EnqueueNetworkInit on the base FlowBuilder like any other staked node
	sanb.EnqueueNetworkInit()

	// if this is upstream staked AN for unstaked ANs, initialize the network to communicate on the unstaked network
	if sanb.supportUnstakedNodes() {
		sanb.enqueueUnstakedNetworkInit()
	}

	sanb.EnqueueMetricsServerInit()

	sanb.RegisterBadgerMetrics()

	sanb.EnqueueTracer()

	return sanb
}

// enqueueUnstakedNetworkInit enqueues the unstaked network component initialized for the staked node
func (sanb *StakedAccessNodeBuilder) enqueueUnstakedNetworkInit() {

	sanb.Component("unstaked network", func(node cmd.NodeBuilder) (module.ReadyDoneAware, error) {

		// NodeID for the staked node on the unstaked network
		// TODO: set a different node ID of the staked access node on the unstaked network
		unstakedNodeID := sanb.NodeID() // currently set the same as the staked NodeID

		// Networking key
		// TODO: set a different networking key of the staked access node on the unstaked network
		unstakedNetworkKey := sanb.NetworkKey()

		// Network Metrics
		// for now we use the empty metrics NoopCollector till we have defined the new unstaked network metrics
		// TODO: define new network metrics for the unstaked network
		unstakedNetworkMetrics := metrics.NewNoopCollector()

		// intialize the LibP2P factory with an empty metrics NoopCollector for now till we have defined the new unstaked
		// network metrics
		libP2PFactory, err := sanb.FlowAccessNodeBuilder.initLibP2PFactory(unstakedNodeID, unstakedNetworkMetrics, unstakedNetworkKey)
		sanb.MustNot(err)

		// use the default validators for the staked access node unstaked networks
		msgValidators := p2p.DefaultValidators(sanb.Logger(), unstakedNodeID)

		// don't need any peer updates since this will be taken care by the DHT discovery mechanism
		peerUpdateInterval := time.Hour

		middleware := sanb.initMiddleware(unstakedNodeID, unstakedNetworkMetrics, libP2PFactory, peerUpdateInterval, msgValidators...)

		// empty list of unstaked network participants since they will be discovered dynamically and are not known upfront
		participants := flow.IdentityList{}

		// topology returns empty list since peers are not known upfront
		top := topology.EmptyListTopology{}

		network, err := sanb.initNetwork(sanb.Me(), unstakedNetworkMetrics, middleware, participants, top)
		sanb.MustNot(err)

		sanb.UnstakedNetwork = network

		logger := sanb.Logger()
		logger.Info().Msgf("unstaked network will run on address: %s", sanb.unstakedNetworkBindAddr)
		return sanb.UnstakedNetwork, err
	})
}
