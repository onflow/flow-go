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

func (uanb *UnstakedAccessNodeBuilder) Initialize() cmd.NodeBuilder {

	uanb.validateParams()

	uanb.enqueueUnstakedNetworkInit()

	uanb.EnqueueMetricsServerInit()

	uanb.RegisterBadgerMetrics()

	uanb.EnqueueTracer()

	uanb.PreInit(uanb.initUnstakedLocal())

	return uanb
}

func (uanb *UnstakedAccessNodeBuilder) validateParams() {

	// for an unstaked access node, the staked access node ID must be provided
	if strings.TrimSpace(uanb.stakedAccessNodeIDHex) == "" {
		uanb.Logger.Fatal().Msg("staked access node ID not specified")
	}

	// and also the unstaked bind address
	if uanb.unstakedNetworkBindAddr == cmd.NotSet {
		uanb.Logger.Fatal().Msg("unstaked bind address not set")
	}
}

// initUnstakedLocal initializes the unstaked node ID, network key and network address
// Currently, it reads a node-info.priv.json like any other node.
// TODO: read the node ID from the special bootstrap files
func (uanb *UnstakedAccessNodeBuilder) initUnstakedLocal() func(builder cmd.NodeBuilder, node *cmd.NodeConfig) {
	return func(builder cmd.NodeBuilder, node *cmd.NodeConfig) {
		// for an unstaked node, set the identity here explicitly since it will not be found in the protocol state
		self := &flow.Identity{
			NodeID:        node.NodeID,
			NetworkPubKey: node.NetworkKey.PublicKey(),
			StakingPubKey: nil,             // no staking key needed for the unstaked node
			Role:          flow.RoleAccess, // unstaked node can only run as an access node
			Address:       uanb.unstakedNetworkBindAddr,
		}

		me, err := local.New(self, nil)
		builder.MustNot(err).Msg("could not initialize local")
		node.Me = me
	}
}

// enqueueUnstakedNetworkInit enqueues the unstaked network component initialized for the unstaked node
func (uanb *UnstakedAccessNodeBuilder) enqueueUnstakedNetworkInit() {

	uanb.Component("unstaked network", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

		// NodeID for the unstaked node on the unstaked network
		unstakedNodeID := node.NodeID

		// Networking key
		unstakedNetworkKey := node.NetworkKey

		// Network Metrics
		// for now we use the empty metrics NoopCollector till we have defined the new unstaked network metrics
		unstakedNetworkMetrics := metrics.NewNoopCollector()

		// intialize the LibP2P factory with an empty metrics NoopCollector for now till we have defined the new unstaked
		// network metrics
		libP2PFactory, err := uanb.FlowAccessNodeBuilder.initLibP2PFactory(unstakedNodeID, unstakedNetworkMetrics, unstakedNetworkKey)
		uanb.MustNot(err)

		// use the default validators for the staked access node unstaked networks
		msgValidators := p2p.DefaultValidators(uanb.Logger, unstakedNodeID)

		// don't need any peer updates since this will be taken care by the DHT discovery mechanism
		peerUpdateInterval := time.Hour

		middleware := uanb.initMiddleware(unstakedNodeID, unstakedNetworkMetrics, libP2PFactory, peerUpdateInterval, msgValidators...)

		// empty list of unstaked network participants since they will be discovered dynamically and are not known upfront
		participants := flow.IdentityList{}

		upstreamANIdentifier, err := flow.HexStringToIdentifier(uanb.stakedAccessNodeIDHex)
		uanb.MustNot(err)

		// topology only consist of the upsteam staked AN
		top := topology.NewFixedListTopology(upstreamANIdentifier)

		network, err := uanb.initNetwork(uanb.Me, unstakedNetworkMetrics, middleware, participants, top)
		uanb.MustNot(err)

		uanb.UnstakedNetwork = network
		uanb.unstakedMiddleware = middleware

		// for an unstaked node, the staked network and middleware is set to the same as the unstaked network and middlware
		uanb.Network = network
		uanb.Middleware = middleware

		uanb.Logger.Info().Msgf("unstaked network will run on address: %s", uanb.unstakedNetworkBindAddr)

		return uanb.UnstakedNetwork, err
	})
}
