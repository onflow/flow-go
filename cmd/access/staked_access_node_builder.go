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

func (builder *StakedAccessNodeBuilder) Initialize() cmd.NodeBuilder {

	// for the staked access node, initialize the network used to communicate with the other staked flow nodes
	// by calling the EnqueueNetworkInit on the base FlowBuilder like any other staked node
	builder.EnqueueNetworkInit()

	// if this is upstream staked AN for unstaked ANs, initialize the network to communicate on the unstaked network
	if builder.ParticipatesInUnstakedNetwork() {
		builder.enqueueUnstakedNetworkInit()
	}

	builder.EnqueueMetricsServerInit()

	builder.RegisterBadgerMetrics()

	builder.EnqueueTracer()

	return builder
}

// enqueueUnstakedNetworkInit enqueues the unstaked network component initialized for the staked node
func (builder *StakedAccessNodeBuilder) enqueueUnstakedNetworkInit() {

	builder.Component("unstaked network", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

		// NodeID for the staked node on the unstaked network
		// TODO: set a different node ID of the staked access node on the unstaked network
		unstakedNodeID := builder.NodeID // currently set the same as the staked NodeID

		// Networking key
		// TODO: set a different networking key of the staked access node on the unstaked network
		unstakedNetworkKey := builder.NetworkKey

		// Network Metrics
		// for now we use the empty metrics NoopCollector till we have defined the new unstaked network metrics
		// TODO: define new network metrics for the unstaked network
		unstakedNetworkMetrics := metrics.NewNoopCollector()

		// intialize the LibP2P factory with an empty metrics NoopCollector for now till we have defined the new unstaked
		// network metrics
		libP2PFactory, err := builder.FlowAccessNodeBuilder.initLibP2PFactory(unstakedNodeID, unstakedNetworkMetrics, unstakedNetworkKey)
		builder.MustNot(err)

		// use the default validators for the staked access node unstaked networks
		msgValidators := p2p.DefaultValidators(builder.Logger, unstakedNodeID)

		// don't need any peer updates since this will be taken care by the DHT discovery mechanism
		peerUpdateInterval := time.Hour

		middleware := builder.initMiddleware(unstakedNodeID, unstakedNetworkMetrics, libP2PFactory, peerUpdateInterval, msgValidators...)

		// empty list of unstaked network participants since they will be discovered dynamically and are not known upfront
		// TODO: this list should be the unstaked addresses of all the staked AN that participate in the unstaked network
		participants := flow.IdentityList{}

		// topology returns empty list since peers are not known upfront
		top := topology.EmptyListTopology{}

		network, err := builder.initNetwork(builder.Me, unstakedNetworkMetrics, middleware, participants, top)
		builder.MustNot(err)

		builder.UnstakedNetwork = network
		builder.unstakedMiddleware = middleware

		node.Logger.Info().Msgf("unstaked network will run on address: %s", builder.unstakedNetworkBindAddr)
		return builder.UnstakedNetwork, err
	})
}
