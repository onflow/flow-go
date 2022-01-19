package node_builder

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

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
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
)

type UnstakedAccessNodeBuilder struct {
	*FlowAccessNodeBuilder
	peerID peer.ID
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

	anb.peerID, err = peer.IDFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("could not get peer ID from public key: %w", err)
	}

	anb.NodeID, err = p2p.NewUnstakedNetworkIDTranslator().GetFlowID(anb.peerID)
	if err != nil {
		return fmt.Errorf("could not get flow node ID: %w", err)
	}

	anb.NodeConfig.NetworkKey = networkingKey // copy the key to NodeConfig
	anb.NodeConfig.StakingKey = nil           // no staking key for the unstaked node

	return nil
}

func (anb *UnstakedAccessNodeBuilder) InitIDProviders() {
	anb.Module("id providers", func(node *cmd.NodeConfig) error {
		idCache, err := p2p.NewProtocolStateIDCache(node.Logger, node.State, anb.ProtocolEvents)
		if err != nil {
			return err
		}

		anb.IdentityProvider = idCache

		anb.IDTranslator = p2p.NewHierarchicalIDTranslator(idCache, p2p.NewUnstakedNetworkIDTranslator())

		// use the default identifier provider
		anb.SyncEngineParticipantsProviderFactory = func() id.IdentifierProvider {
			return id.NewCustomIdentifierProvider(func() flow.IdentifierList {
				var result flow.IdentifierList

				pids := anb.LibP2PNode.GetPeersForProtocol(unicast.FlowProtocolID(anb.RootBlock.ID()))

				for _, pid := range pids {
					// exclude own Identifier
					if pid == anb.peerID {
						continue
					}

					if flowID, err := anb.IDTranslator.GetFlowID(pid); err != nil {
						anb.Logger.Err(err).Str("peer", pid.Pretty()).Msg("failed to translate to Flow ID")
					} else {
						result = append(result, flowID)
					}
				}

				return result
			})
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

	var pis []peer.AddrInfo
	for _, b := range builder.bootstrapIdentities {
		pi, err := p2p.PeerAddressInfo(*b)
		if err != nil {
			return nil, fmt.Errorf("could not extract peer address info from bootstrap identity %v: %w", b, err)
		}
		pis = append(pis, pi)
	}

	psOpts := append(p2p.DefaultPubsubOptions(p2p.DefaultMaxPubSubMsgSize),
		func(_ context.Context, h host.Host) (pubsub.Option, error) {
			return pubsub.WithSubscriptionFilter(p2p.NewRoleBasedFilter(
				h.ID(), builder.IdentityProvider,
			)), nil
		},
		// Note: using the WithDirectPeers option will automatically store these addresses
		// as permanent addresses in the Peerstore and try to connect to them when the
		// PubSubRouter starts up
		p2p.PubSubOptionWrapper(pubsub.WithDirectPeers(pis)),
	)

	return func(ctx context.Context) (*p2p.Node, error) {
		libp2pNode, err := p2p.NewDefaultLibP2PNodeBuilder(nodeID, builder.BaseConfig.BindAddr, networkKey).
			SetSporkID(builder.SporkID).
			SetConnectionManager(connManager).
			// unlike the staked side of the network where currently all the node addresses are known upfront,
			// for the unstaked side of the network, the  nodes need to discover each other using DHT Discovery.
			SetDHTOptions(dhtOptions...).
			SetLogger(builder.Logger).
			SetResolver(resolver).
			SetPubsubOptions(psOpts...).
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
func (anb *UnstakedAccessNodeBuilder) initUnstakedLocal() func(node *cmd.NodeConfig) error {
	return func(node *cmd.NodeConfig) error {
		// for an unstaked node, set the identity here explicitly since it will not be found in the protocol state
		self := &flow.Identity{
			NodeID:        node.NodeID,
			NetworkPubKey: node.NetworkKey.PublicKey(),
			StakingPubKey: nil,             // no staking key needed for the unstaked node
			Role:          flow.RoleAccess, // unstaked node can only run as an access node
			Address:       anb.BindAddr,
		}

		var err error
		node.Me, err = local.NewNoKey(self)
		if err != nil {
			return fmt.Errorf("could not initialize local: %w", err)
		}
		return nil
	}
}

// enqueueMiddleware enqueues the creation of the network middleware
// this needs to be done before sync engine participants module
func (anb *UnstakedAccessNodeBuilder) enqueueMiddleware() {
	anb.
		Module("network middleware", func(node *cmd.NodeConfig) error {

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
func (anb *UnstakedAccessNodeBuilder) Build() (cmd.Node, error) {
	anb.BuildConsensusFollower()
	return anb.FlowAccessNodeBuilder.Build()
}

// enqueueUnstakedNetworkInit enqueues the unstaked network component initialized for the unstaked node
func (anb *UnstakedAccessNodeBuilder) enqueueUnstakedNetworkInit() {

	anb.Component("unstaked network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

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
	anb.Component("upstream connector", func(_ *cmd.NodeConfig) (module.ReadyDoneAware, error) {
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
		anb.SporkID,
		p2p.DefaultUnicastTimeout,
		anb.IDTranslator,
		p2p.WithMessageValidators(validators...),
		p2p.WithConnectionGating(false),
		// no peer manager
		// use default identifier provider
	)

	return anb.Middleware
}
