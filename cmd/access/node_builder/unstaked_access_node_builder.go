package node_builder

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/converter"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/utils/io"
)

type UnstakedAccessNodeBuilder struct {
	*FlowAccessNodeBuilder
	peerID peer.ID
}

func NewUnstakedAccessNodeBuilder(builder *FlowAccessNodeBuilder) *UnstakedAccessNodeBuilder {
	// the unstaked access node gets a version of the root snapshot file that does not contain any node addresses
	// hence skip all the root snapshot validations that involved an identity address
	builder.SkipNwAddressBasedValidations = true
	return &UnstakedAccessNodeBuilder{
		FlowAccessNodeBuilder: builder,
	}
}

func (builder *UnstakedAccessNodeBuilder) initNodeInfo() error {
	// use the networking key that has been passed in the config, or load from the configured file
	networkingKey := builder.AccessNodeConfig.NetworkKey
	if networkingKey == nil {
		var err error
		networkingKey, err = loadNetworkingKey(builder.observerNetworkingKeyPath)
		if err != nil {
			return fmt.Errorf("could not load networking private key: %w", err)
		}
	}

	pubKey, err := keyutils.LibP2PPublicKeyFromFlow(networkingKey.PublicKey())
	if err != nil {
		return fmt.Errorf("could not load networking public key: %w", err)
	}

	builder.peerID, err = peer.IDFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("could not get peer ID from public key: %w", err)
	}

	builder.NodeID, err = p2p.NewUnstakedNetworkIDTranslator().GetFlowID(builder.peerID)
	if err != nil {
		return fmt.Errorf("could not get flow node ID: %w", err)
	}

	builder.NodeConfig.NetworkKey = networkingKey // copy the key to NodeConfig
	builder.NodeConfig.StakingKey = nil           // no staking key for the unstaked node

	return nil
}

func (builder *UnstakedAccessNodeBuilder) InitIDProviders() {
	builder.Module("id providers", func(node *cmd.NodeConfig) error {
		idCache, err := p2p.NewProtocolStateIDCache(node.Logger, node.State, builder.ProtocolEvents)
		if err != nil {
			return err
		}

		builder.IdentityProvider = idCache

		builder.IDTranslator = p2p.NewHierarchicalIDTranslator(idCache, p2p.NewUnstakedNetworkIDTranslator())

		// use the default identifier provider
		builder.SyncEngineParticipantsProviderFactory = func() id.IdentifierProvider {
			return id.NewCustomIdentifierProvider(func() flow.IdentifierList {
				var result flow.IdentifierList

				pids := builder.LibP2PNode.GetPeersForProtocol(unicast.FlowProtocolID(builder.SporkID))

				for _, pid := range pids {
					// exclude own Identifier
					if pid == builder.peerID {
						continue
					}

					if flowID, err := builder.IDTranslator.GetFlowID(pid); err != nil {
						builder.Logger.Err(err).Str("peer", pid.Pretty()).Msg("failed to translate to Flow ID")
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

func (builder *UnstakedAccessNodeBuilder) Initialize() error {
	if err := builder.deriveBootstrapPeerIdentities(); err != nil {
		return err
	}

	if err := builder.validateParams(); err != nil {
		return err
	}

	if err := builder.initNodeInfo(); err != nil {
		return err
	}

	builder.InitIDProviders()

	builder.enqueueMiddleware()

	builder.enqueueUnstakedNetworkInit()

	builder.enqueueConnectWithStakedAN()

	if builder.BaseConfig.MetricsEnabled {
		builder.EnqueueMetricsServerInit()
		if err := builder.RegisterBadgerMetrics(); err != nil {
			return err
		}
	}

	builder.PreInit(builder.initUnstakedLocal())

	return nil
}

func (builder *UnstakedAccessNodeBuilder) validateParams() error {
	if builder.BaseConfig.BindAddr == cmd.NotSet || builder.BaseConfig.BindAddr == "" {
		return errors.New("bind address not specified")
	}
	if builder.AccessNodeConfig.NetworkKey == nil && builder.AccessNodeConfig.observerNetworkingKeyPath == cmd.NotSet {
		return errors.New("networking key not provided")
	}
	if len(builder.bootstrapIdentities) > 0 {
		return nil
	}
	if len(builder.bootstrapNodeAddresses) == 0 {
		return errors.New("no bootstrap node address provided")
	}
	if len(builder.bootstrapNodeAddresses) != len(builder.bootstrapNodePublicKeys) {
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
func (builder *UnstakedAccessNodeBuilder) initLibP2PFactory(networkKey crypto.PrivateKey) p2p.LibP2PFactoryFunc {
	return func(ctx context.Context) (*p2p.Node, error) {
		var pis []peer.AddrInfo

		for _, b := range builder.bootstrapIdentities {
			pi, err := p2p.PeerAddressInfo(*b)

			if err != nil {
				return nil, fmt.Errorf("could not extract peer address info from bootstrap identity %v: %w", b, err)
			}

			pis = append(pis, pi)
		}

		node, err := p2p.NewNodeBuilder(builder.Logger, builder.BaseConfig.BindAddr, networkKey, builder.SporkID).
			SetSubscriptionFilter(
				p2p.NewRoleBasedFilter(
					p2p.UnstakedRole, builder.IdentityProvider,
				),
			).
			SetRoutingSystem(func(ctx context.Context, h host.Host) (routing.Routing, error) {
				return p2p.NewDHT(ctx, h, unicast.FlowPublicDHTProtocolID(builder.SporkID),
					p2p.AsClient(),
					dht.BootstrapPeers(pis...),
				)
			}).
			SetPubSub(pubsub.NewGossipSub).
			Build(ctx)

		if err != nil {
			return nil, err
		}

		builder.LibP2PNode = node

		return builder.LibP2PNode, nil
	}
}

// initUnstakedLocal initializes the unstaked node ID, network key and network address
// Currently, it reads a node-info.priv.json like any other node.
// TODO: read the node ID from the special bootstrap files
func (builder *UnstakedAccessNodeBuilder) initUnstakedLocal() func(node *cmd.NodeConfig) error {
	return func(node *cmd.NodeConfig) error {
		// for an unstaked node, set the identity here explicitly since it will not be found in the protocol state
		self := &flow.Identity{
			NodeID:        node.NodeID,
			NetworkPubKey: node.NetworkKey.PublicKey(),
			StakingPubKey: nil,             // no staking key needed for the unstaked node
			Role:          flow.RoleAccess, // unstaked node can only run as an access node
			Address:       builder.BindAddr,
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
func (builder *UnstakedAccessNodeBuilder) enqueueMiddleware() {
	builder.
		Module("network middleware", func(node *cmd.NodeConfig) error {

			// NodeID for the unstaked node on the unstaked network
			unstakedNodeID := node.NodeID

			// Networking key
			unstakedNetworkKey := node.NetworkKey

			libP2PFactory := builder.initLibP2PFactory(unstakedNetworkKey)

			msgValidators := unstakedNetworkMsgValidators(node.Logger, node.IdentityProvider, unstakedNodeID)

			builder.initMiddleware(unstakedNodeID, node.Metrics.Network, libP2PFactory, msgValidators...)

			return nil
		})
}

// Build enqueues the sync engine and the follower engine for the unstaked access node.
// Currently, the unstaked AN only runs the follower engine.
func (builder *UnstakedAccessNodeBuilder) Build() (cmd.Node, error) {
	builder.BuildConsensusFollower()
	return builder.FlowAccessNodeBuilder.Build()
}

// enqueueUnstakedNetworkInit enqueues the unstaked network component initialized for the unstaked node
func (builder *UnstakedAccessNodeBuilder) enqueueUnstakedNetworkInit() {

	builder.Component("unstaked network", func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		// topology is nil since its automatically managed by libp2p
		network, err := builder.initNetwork(builder.Me, builder.Metrics.Network, builder.Middleware, nil)
		if err != nil {
			return nil, err
		}

		builder.Network = converter.NewNetwork(network, engine.SyncCommittee, engine.PublicSyncCommittee)

		builder.Logger.Info().Msgf("network will run on address: %s", builder.BindAddr)

		idEvents := gadgets.NewIdentityDeltas(builder.Middleware.UpdateNodeAddresses)
		builder.ProtocolEvents.AddConsumer(idEvents)

		return builder.Network, nil
	})
}

// enqueueConnectWithStakedAN enqueues the upstream connector component which connects the libp2p host of the unstaked
// AN with the staked AN.
// Currently, there is an issue with LibP2P stopping advertisements of subscribed topics if no peers are connected
// (https://github.com/libp2p/go-libp2p-pubsub/issues/442). This means that an unstaked AN could end up not being
// discovered by other unstaked ANs if it subscribes to a topic before connecting to the staked AN. Hence, the need
// of an explicit connect to the staked AN before the node attempts to subscribe to topics.
func (builder *UnstakedAccessNodeBuilder) enqueueConnectWithStakedAN() {
	builder.Component("upstream connector", func(_ *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		return newUpstreamConnector(builder.bootstrapIdentities, builder.LibP2PNode, builder.Logger), nil
	})
}

// initMiddleware creates the network.Middleware implementation with the libp2p factory function, metrics, peer update
// interval, and validators. The network.Middleware is then passed into the initNetwork function.
func (builder *UnstakedAccessNodeBuilder) initMiddleware(nodeID flow.Identifier,
	networkMetrics module.NetworkMetrics,
	factoryFunc p2p.LibP2PFactoryFunc,
	validators ...network.MessageValidator) network.Middleware {

	builder.Middleware = p2p.NewMiddleware(
		builder.Logger,
		factoryFunc,
		nodeID,
		networkMetrics,
		builder.SporkID,
		p2p.DefaultUnicastTimeout,
		builder.IDTranslator,
		p2p.WithMessageValidators(validators...),
		// no peer manager
		// use default identifier provider
	)

	return builder.Middleware
}

func loadNetworkingKey(path string) (crypto.PrivateKey, error) {
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read networking key (path=%s): %w", path, err)
	}

	keyBytes, err := hex.DecodeString(strings.Trim(string(data), "\n "))
	if err != nil {
		return nil, fmt.Errorf("could not hex decode networking key (path=%s): %w", path, err)
	}

	networkingKey, err := crypto.DecodePrivateKey(crypto.ECDSASecp256k1, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("could not decode networking key (path=%s): %w", path, err)
	}

	return networkingKey, nil
}
