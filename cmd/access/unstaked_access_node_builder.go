package main

import (
	"context"
	"encoding/json"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/metrics"
)

type UnstakedAccessNodeBuilder struct {
	bootstrapIdentites flow.IdentityList // the identity list of bootstrap peers the node uses to discover other nodes
	*FlowAccessNodeBuilder
}

func NewUnstakedAccessNodeBuilder(anb *FlowAccessNodeBuilder) *UnstakedAccessNodeBuilder {
	return &UnstakedAccessNodeBuilder{
		FlowAccessNodeBuilder: anb,
	}
}

func (builder *UnstakedAccessNodeBuilder) Initialize() cmd.NodeBuilder {

	ctx, cancel := context.WithCancel(context.Background())
	builder.Cancel = cancel

	builder.validateParams()

	builder.deriveBootstrapPeerIdentities()

	builder.enqueueUnstakedNetworkInit(ctx)

	builder.enqueueConnectWithStakedAN(ctx)

	builder.EnqueueMetricsServerInit()

	builder.RegisterBadgerMetrics()

	builder.EnqueueTracer()

	builder.PreInit(builder.initUnstakedLocal())

	return builder
}

func (builder *UnstakedAccessNodeBuilder) validateParams() {

	// for an unstaked access node, the unstaked network bind address must be provided
	if builder.unstakedNetworkBindAddr == cmd.NotSet {
		builder.Logger.Fatal().Msg("unstaked bind address not set")
	}

	if len(builder.bootstrapNodePublicKeys) != len(builder.bootstrapNodeAddresses) {
		builder.Logger.Fatal().Msg("number of networking public keys and staked access node addresses should match")
	}
}

// deriveBootstrapPeerIdentities derives the Flow Identity of the bootstreap peers from the parameters.
// These are the identity of the staked and unstaked AN also acting as the DHT bootstrap server
func (builder *UnstakedAccessNodeBuilder) deriveBootstrapPeerIdentities() {

	builder.bootstrapIdentites = make([]*flow.Identity, len(builder.bootstrapNodeAddresses))
	for i, address := range builder.bootstrapNodeAddresses {
		key := builder.bootstrapNodePublicKeys[i]

		// networking public key
		var networkKey encodable.NetworkPubKey
		err := json.Unmarshal([]byte(key), &networkKey)
		builder.MustNot(err)

		// create the identity of the peer by setting only the relevant fields
		id := &flow.Identity{
			NodeID:        flow.ZeroID, // the NodeID is the hash of the staking key and for the unstaked network it does not apply
			Address:       address,
			Role:          flow.RoleAccess, // the upstream node has to be an access node
			NetworkPubKey: networkKey,
		}
		builder.bootstrapIdentites[i] = id
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
func (builder *UnstakedAccessNodeBuilder) enqueueUnstakedNetworkInit(ctx context.Context) {

	builder.Component("unstaked network", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

		// NodeID for the unstaked node on the unstaked network
		unstakedNodeID := node.NodeID

		// Networking key
		unstakedNetworkKey := node.NetworkKey

		libP2PFactory, err := builder.FlowAccessNodeBuilder.initLibP2PFactory(ctx, unstakedNodeID, unstakedNetworkKey)
		builder.MustNot(err)

		msgValidators := unstakedNetworkMsgValidators(unstakedNodeID)

		// Network Metrics
		// for now we use the empty metrics NoopCollector till we have defined the new unstaked network metrics
		unstakedNetworkMetrics := metrics.NewNoopCollector()

		middleware := builder.initMiddleware(unstakedNodeID, unstakedNetworkMetrics, libP2PFactory,
			msgValidators...)

		// empty list of unstaked network participants since they will be discovered dynamically and are not known upfront
		participants := flow.IdentityList{}

		// topology is nil since its automatically managed by libp2p
		network, err := builder.initNetwork(builder.Me, unstakedNetworkMetrics, middleware, participants, nil)
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

// enqueueConnectWithStakedAN enqueues the upstream connector component which connects the libp2p host of the unstaked
// AN with the staked AN.
// Currently, there is an issue with LibP2P stopping advertisements of subscribed topics if no peers are connected
// (https://github.com/libp2p/go-libp2p-pubsub/issues/442). This means that an unstaked AN could end up not being
// discovered by other unstaked ANs if it subscribes to a topic before connecting to the staked AN. Hence, the need
// of an explicit connect to the staked AN before the node attempts to subscribe to topics.
func (builder *UnstakedAccessNodeBuilder) enqueueConnectWithStakedAN(ctx context.Context) {
	builder.Component("unstaked network", func(_ cmd.NodeBuilder, _ *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return newUpstreamConnector(ctx, builder.bootstrapIdentites, builder.UnstakedLibP2PNode, builder.Logger), nil
	})
}
