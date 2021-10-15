package node_builder

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/converter"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
)

type UnstakedAccessNodeBuilder struct {
	*FlowAccessNodeBuilder
}

func NewUnstakedAccessNodeBuilder(anb *FlowAccessNodeBuilder) *UnstakedAccessNodeBuilder {
	// the unstaked access node gets a version of the root snapshot file that does not contain any node addresses
	// hence skip all the root snapshot validations that involved an identity address
	anb.SkipNwAddressBasedValidations = true
	return &UnstakedAccessNodeBuilder{
		FlowAccessNodeBuilder: anb,
	}
}

func (anb *UnstakedAccessNodeBuilder) initNodeInfo() error {
	// use the networking key that has been passed in the config
	networkingKey := anb.AccessNodeConfig.NetworkKey
	pubKey, err := keyutils.LibP2PPublicKeyFromFlow(networkingKey.PublicKey())
	if err != nil {
		return fmt.Errorf("could not load networking public key: %w", err)
	}

	peerID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("could not get peer ID from public key: %w", err)
	}

	anb.NodeID, err = p2p.NewUnstakedNetworkIDTranslator().GetFlowID(peerID)
	if err != nil {
		return fmt.Errorf("could not get flow node ID: %w", err)
	}

	anb.NodeConfig.NetworkKey = networkingKey // copy the key to NodeConfig
	anb.NodeConfig.StakingKey = nil           // no staking key for the unstaked node

	return nil
}

func (anb *UnstakedAccessNodeBuilder) InitIDProviders() {
	anb.Module("id providers", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
		idCache, err := p2p.NewProtocolStateIDCache(node.Logger, node.State, anb.ProtocolEvents)
		if err != nil {
			return err
		}

		anb.IdentityProvider = idCache

		anb.IDTranslator = p2p.NewHierarchicalIDTranslator(idCache, p2p.NewUnstakedNetworkIDTranslator())

		// use the default identifier provider
		anb.SyncEngineParticipantsProviderFactory = func() id.IdentifierProvider {

			// use the middleware that should have now been initialized
			middleware, ok := anb.Middleware.(*p2p.Middleware)
			if !ok {
				anb.Logger.Fatal().Msg("middleware was of unexpected type")
			}
			return middleware.IdentifierProvider()
		}

		return nil
	})
}

func (anb *UnstakedAccessNodeBuilder) Initialize() error {
	if err := anb.deriveBootstrapPeerIdentities(); err != nil {
		return err
	}

	if err := anb.validateParams(); err != nil {
		return err
	}

	if err := anb.initNodeInfo(); err != nil {
		return err
	}

	anb.InitIDProviders()

	anb.enqueueMiddleware()

	anb.enqueueUnstakedNetworkInit()

	anb.enqueueConnectWithStakedAN()

	anb.PreInit(anb.initUnstakedLocal())

	return nil
}

// deriveBootstrapPeerIdentities derives the Flow Identity of the bootstrap peers from the parameters.
// These are the identities of the staked and unstaked ANs also acting as the DHT bootstrap server
func (builder *FlowAccessNodeBuilder) deriveBootstrapPeerIdentities() error {
	// if bootstrap identities already provided (as part of alternate initialization as a library the skip reading command
	// line params)
	if builder.bootstrapIdentities != nil {
		return nil
	}
	ids, err := BootstrapIdentities(builder.bootstrapNodeAddresses, builder.bootstrapNodePublicKeys)
	if err != nil {
		return fmt.Errorf("failed to derive bootstrap peer identities: %w", err)
	}
	builder.bootstrapIdentities = ids

	return nil
}

func (anb *UnstakedAccessNodeBuilder) validateParams() error {
	if anb.BaseConfig.BindAddr == cmd.NotSet || anb.BaseConfig.BindAddr == "" {
		return errors.New("bind address not specified")
	}
	if anb.AccessNodeConfig.NetworkKey == nil {
		return errors.New("networking key not provided")
	}
	if len(anb.bootstrapIdentities) > 0 {
		return nil
	}
	if len(anb.bootstrapNodeAddresses) == 0 {
		return errors.New("no bootstrap node address provided")
	}
	if len(anb.bootstrapNodeAddresses) != len(anb.bootstrapNodePublicKeys) {
		return errors.New("number of bootstrap node addresses and public keys should match")
	}
	return nil
}

// initLibP2PFactory creates the LibP2P factory function for the given node ID and network key for the unstaked node.
// The factory function is later passed into the initMiddleware function to eventually instantiate the p2p.LibP2PNode instance
// The LibP2P host is created with the following options:
// 		DHT as client and seeded with the given bootstrap peers
// 		The specified bind address as the listen address
// 		The passed in private key as the libp2p key
//		No connection gater
// 		No connection manager
// 		Default libp2p pubsub options
func (builder *UnstakedAccessNodeBuilder) initLibP2PFactory(nodeID flow.Identifier, networkKey crypto.PrivateKey) (p2p.LibP2PFactoryFunc, error) {

	// the unstaked nodes act as the DHT clients
	dhtOptions := []dht.Option{p2p.AsServer(false)}

	// seed the DHT with the boostrap identities
	bootstrapPeersOpt, err := p2p.WithBootstrapPeers(builder.bootstrapIdentities)
	if err != nil {
		return nil, err
	}
	dhtOptions = append(dhtOptions, bootstrapPeersOpt)

	connManager := p2p.NewConnManager(builder.Logger, builder.Metrics.Network, p2p.TrackUnstakedConnections(builder.IdentityProvider))

	resolver := dns.NewResolver(builder.Metrics.Network, dns.WithTTL(builder.BaseConfig.DNSCacheTTL))

	return func(ctx context.Context) (*p2p.Node, error) {
		libp2pNode, err := p2p.NewDefaultLibP2PNodeBuilder(nodeID, builder.BaseConfig.BindAddr, networkKey).
			SetRootBlockID(builder.RootBlock.ID()).
			SetConnectionManager(connManager).
			// unlike the staked side of the network where currently all the node addresses are known upfront,
			// for the unstaked side of the network, the  nodes need to discover each other using DHT Discovery.
			SetDHTOptions(dhtOptions...).
			SetLogger(builder.Logger).
			SetResolver(resolver).
			SetStreamCompressor(p2p.WithGzipCompression).
			Build(ctx)
		if err != nil {
			return nil, err
		}
		builder.LibP2PNode = libp2pNode
		return builder.LibP2PNode, nil
	}, nil
}

// initUnstakedLocal initializes the unstaked node ID, network key and network address
// Currently, it reads a node-info.priv.json like any other node.
// TODO: read the node ID from the special bootstrap files
func (anb *UnstakedAccessNodeBuilder) initUnstakedLocal() func(builder cmd.NodeBuilder, node *cmd.NodeConfig) {
	return func(_ cmd.NodeBuilder, node *cmd.NodeConfig) {
		// for an unstaked node, set the identity here explicitly since it will not be found in the protocol state
		self := &flow.Identity{
			NodeID:        node.NodeID,
			NetworkPubKey: node.NetworkKey.PublicKey(),
			StakingPubKey: nil,             // no staking key needed for the unstaked node
			Role:          flow.RoleAccess, // unstaked node can only run as an access node
			Address:       anb.BindAddr,
		}

		me, err := local.New(self, nil)
		anb.MustNot(err).Msg("could not initialize local")
		node.Me = me
	}
}

// enqueueMiddleware enqueues the creation of the network middleware
// this needs to be done before sync engine participants module
func (anb *UnstakedAccessNodeBuilder) enqueueMiddleware() {
	anb.
		Module("network middleware", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) error {

			// NodeID for the unstaked node on the unstaked network
			unstakedNodeID := node.NodeID

			// Networking key
			unstakedNetworkKey := node.NetworkKey

			// Network Metrics
			// for now we use the empty metrics NoopCollector till we have defined the new unstaked network metrics
			unstakedNetworkMetrics := metrics.NewNoopCollector()

			libP2PFactory, err := anb.initLibP2PFactory(unstakedNodeID, unstakedNetworkKey)
			if err != nil {
				return err
			}

			msgValidators := unstakedNetworkMsgValidators(node.Logger, node.IdentityProvider, unstakedNodeID)

			anb.initMiddleware(unstakedNodeID, unstakedNetworkMetrics, libP2PFactory, msgValidators...)

			return nil
		})
}

// Build enqueues the sync engine and the follower engine for the unstaked access node.
// Currently, the unstaked AN only runs the follower engine.
func (anb *UnstakedAccessNodeBuilder) Build() AccessNodeBuilder {
	anb.FlowAccessNodeBuilder.BuildConsensusFollower()
	return anb
}

// enqueueUnstakedNetworkInit enqueues the unstaked network component initialized for the unstaked node
func (anb *UnstakedAccessNodeBuilder) enqueueUnstakedNetworkInit() {

	anb.Component("unstaked network", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

		// Network Metrics
		// for now we use the empty metrics NoopCollector till we have defined the new unstaked network metrics
		unstakedNetworkMetrics := metrics.NewNoopCollector()

		// topology is nil since its automatically managed by libp2p
		network, err := anb.initNetwork(anb.Me, unstakedNetworkMetrics, anb.Middleware, nil)
		if err != nil {
			return nil, err
		}

		anb.Network = converter.NewNetwork(network, engine.SyncCommittee, engine.PublicSyncCommittee)

		anb.Logger.Info().Msgf("network will run on address: %s", anb.BindAddr)

		idEvents := gadgets.NewIdentityDeltas(anb.Middleware.UpdateNodeAddresses)
		anb.ProtocolEvents.AddConsumer(idEvents)

		return anb.Network, nil
	})
}

// enqueueConnectWithStakedAN enqueues the upstream connector component which connects the libp2p host of the unstaked
// AN with the staked AN.
// Currently, there is an issue with LibP2P stopping advertisements of subscribed topics if no peers are connected
// (https://github.com/libp2p/go-libp2p-pubsub/issues/442). This means that an unstaked AN could end up not being
// discovered by other unstaked ANs if it subscribes to a topic before connecting to the staked AN. Hence, the need
// of an explicit connect to the staked AN before the node attempts to subscribe to topics.
func (anb *UnstakedAccessNodeBuilder) enqueueConnectWithStakedAN() {
	anb.Component("upstream connector", func(_ cmd.NodeBuilder, _ *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return newUpstreamConnector(anb.bootstrapIdentities, anb.LibP2PNode, anb.Logger), nil
	})
}

// initMiddleware creates the network.Middleware implementation with the libp2p factory function, metrics, peer update
// interval, and validators. The network.Middleware is then passed into the initNetwork function.
func (anb *UnstakedAccessNodeBuilder) initMiddleware(nodeID flow.Identifier,
	networkMetrics module.NetworkMetrics,
	factoryFunc p2p.LibP2PFactoryFunc,
	validators ...network.MessageValidator) network.Middleware {

	anb.Middleware = p2p.NewMiddleware(
		anb.Logger,
		factoryFunc,
		nodeID,
		networkMetrics,
		anb.RootBlock.ID(),
		p2p.DefaultUnicastTimeout,
		false, // no connection gating for the unstaked nodes
		anb.IDTranslator,
		p2p.WithMessageValidators(validators...),
		// no peer manager
		// use default identifier provider
	)

	return anb.Middleware
}
