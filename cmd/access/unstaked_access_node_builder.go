package main

import (
	"strings"
	"time"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
)

type UnstakedAccessNodeBuilder struct {
	*FlowAccessNodeBuilder
}

func UnstakedAccessNode(anb *FlowAccessNodeBuilder) *UnstakedAccessNodeBuilder {
	return &UnstakedAccessNodeBuilder{
		FlowAccessNodeBuilder: anb,
	}
}

func (builder *UnstakedAccessNodeBuilder) Initialize() cmd.NodeBuilder {

	builder.validateParams()

	builder.enqueueUnstakedNetworkInit()

	builder.EnqueueMetricsServerInit()

	builder.RegisterBadgerMetrics()

	builder.EnqueueTracer()

	builder.PreInit(builder.initUnstakedLocal())

	return builder
}

func (builder *UnstakedAccessNodeBuilder) validateParams() {

	// for an unstaked access node, the staked access node ID must be provided
	if strings.TrimSpace(builder.stakedAccessNodeIDHex) == "" {
		builder.Logger.Fatal().Msg("staked access node ID not specified")
	}

	// and also the unstaked bind address
	if builder.unstakedNetworkBindAddr == cmd.NotSet {
		builder.Logger.Fatal().Msg("unstaked bind address not set")
	}
}

// initUnstakedLocal initializes the unstaked node ID, network key and network address
// Currently, it reads a node-info.priv.json like any other node.
// TODO: read the node ID from the special bootstrap files
func (builder *UnstakedAccessNodeBuilder) initUnstakedLocal() func(builder cmd.NodeBuilder, node *cmd.NodeConfig) {
	return func(_ cmd.NodeBuilder, node *cmd.NodeConfig) {
		// for an unstaked node, set the identity here explicitly since it will not be found in the protocol state
		self := &flow.Identity{
			NodeID:        node.NodeID,
			NetworkPubKey: node.NetworkKey.PublicKey(),
			StakingPubKey: nil,             // no staking key needed for the unstaked node
			Role:          flow.RoleAccess, // unstaked node can only run as an access node
			Address:       builder.unstakedNetworkBindAddr,
		}

		me, err := local.New(self, nil)
		builder.MustNot(err).Msg("could not initialize local")
		node.Me = me
	}
}

// enqueueUnstakedNetworkInit enqueues the unstaked network component initialized for the unstaked node
func (builder *UnstakedAccessNodeBuilder) enqueueUnstakedNetworkInit() {

	builder.Component("unstaked network", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

		// NodeID for the unstaked node on the unstaked network
		unstakedNodeID := node.NodeID

		// Networking key
		unstakedNetworkKey := node.NetworkKey

		// Network Metrics
		// for now we use the empty metrics NoopCollector till we have defined the new unstaked network metrics
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
		participants := flow.IdentityList{}

		upstreamANIdentifier, err := flow.HexStringToIdentifier(builder.stakedAccessNodeIDHex)
		builder.MustNot(err)

		// topology only consist of the upsteam staked AN
		top := topology.NewFixedListTopology(upstreamANIdentifier)

		network, err := builder.initNetwork(builder.Me, unstakedNetworkMetrics, middleware, participants, top)
		builder.MustNot(err)

		builder.UnstakedNetwork = network
		builder.unstakedMiddleware = middleware

		// for an unstaked node, the staked network and middleware is set to the same as the unstaked network and middlware
		builder.Network = network
		builder.Middleware = middleware

		builder.Logger.Info().Msgf("unstaked network will run on address: %s", builder.unstakedNetworkBindAddr)

		return builder.UnstakedNetwork, err
	})
}
