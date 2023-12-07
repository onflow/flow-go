package corruptlibp2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/cmd"
	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// InitCorruptLibp2pNode initializes and returns a corrupt libp2p node that should only be used for BFT testing in
// the BFT testnet. This node is corrupt in the sense that it uses a forked version of the go-libp2p-pubsub library and
// is not compatible with the go-libp2p-pubsub library used by the other nodes in the network. This node should only be
// used for testing purposes.
// Args:
// - log: logger
// - chainID: chain id of the network this node is being used for (should be BFT testnet)
// - address: address of the node in the form of /ip4/ ... /tcp/ ... /p2p/ ... (see libp2p documentation for more info)
// - flowKey: private key of the node used for signing messages and establishing secure connections
// - sporkId: spork id of the network this node is being used for.
// - idProvider: identity provider used for translating peer ids to flow ids.
// - metricsCfg: metrics configuration used for initializing the metrics collector
// - resolver: resolver used for resolving multiaddresses to ip addresses
// - role: role of the node (a valid Flow role).
// - connGaterCfg: connection gater configuration used for initializing the connection gater
// - peerManagerCfg: peer manager configuration used for initializing the peer manager
// - uniCfg: unicast configuration used for initializing the unicast
// - gossipSubCfg: gossipsub configuration used for initializing the gossipsub
// - topicValidatorDisabled: whether or not topic validator is disabled
// - withMessageSigning: whether or not message signing is enabled
// - withStrictSignatureVerification: whether or not strict signature verification is enabled
// Returns:
// - p2p.LibP2PNode: initialized corrupt libp2p node
// - error: error if any. Any error returned from this function is fatal.
func InitCorruptLibp2pNode(
	log zerolog.Logger,
	chainID flow.ChainID,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	metricsCfg module.NetworkMetrics,
	resolver madns.BasicResolver,
	role string,
	connGaterCfg *p2pconfig.ConnectionGaterConfig,
	peerManagerCfg *p2pconfig.PeerManagerConfig,
	uniCfg *p2pconfig.UnicastConfig,
	netConfig *netconf.Config,
	disallowListCacheCfg *p2p.DisallowListCacheConfig,
	topicValidatorDisabled,
	withMessageSigning,
	withStrictSignatureVerification bool,
) (p2p.LibP2PNode, error) {
	if chainID != flow.BftTestnet {
		panic("illegal chain id for using corrupt libp2p node")
	}

	metCfg := &p2pconfig.MetricsConfig{
		HeroCacheFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		Metrics:          metricsCfg,
	}

	dhtActivationStatus, err := cmd.DhtSystemActivationStatus(role)
	if err != nil {
		return nil, fmt.Errorf("could not get dht system activation status: %w", err)
	}
	builder, err := p2pbuilder.DefaultNodeBuilder(
		log,
		address,
		network.PrivateNetwork,
		flowKey,
		sporkId,
		idProvider,
		metCfg,
		resolver,
		role,
		connGaterCfg,
		peerManagerCfg,
		&netConfig.GossipSub,
		&netConfig.ResourceManager,
		uniCfg,
		&netConfig.ConnectionManager,
		disallowListCacheCfg,
		dhtActivationStatus)

	if err != nil {
		return nil, fmt.Errorf("could not create corrupt libp2p node builder: %w", err)
	}
	if topicValidatorDisabled {
		builder.OverrideNodeConstructor(func(config *p2p.NodeConfig) (p2p.LibP2PNode, error) {
			node, err := p2pnode.NewNode(&p2p.NodeConfig{
				Logger:               config.Logger,
				Host:                 config.Host,
				PeerManager:          config.PeerManager,
				DisallowListCacheCfg: disallowListCacheCfg,
			})

			if err != nil {
				return nil, fmt.Errorf("could not create libp2p node part of the corrupt libp2p: %w", err)
			}

			return &CorruptP2PNode{Node: node, logger: config.Logger.With().Str("component", "corrupt_libp2p").Logger(), codec: cbor.NewCodec()}, nil
		})
	}

	overrideWithCorruptGossipSub(
		builder,
		WithMessageSigning(withMessageSigning),
		WithStrictSignatureVerification(withStrictSignatureVerification))
	return builder.Build()
}

// CorruptGossipSubFactory returns a factory function that creates a new instance of the forked gossipsub module from
// github.com/yhassanzadeh13/go-libp2p-pubsub for the purpose of BFT testing and attack vector implementation.
func CorruptGossipSubFactory(routerOpts ...func(*corrupt.GossipSubRouter)) p2p.GossipSubFactoryFunc {
	factory := func(
		ctx context.Context,
		logger zerolog.Logger,
		host host.Host,
		cfg p2p.PubSubAdapterConfig,
		clusterChangeConsumer p2p.CollectionClusterChangesConsumer) (p2p.PubSubAdapter, error) {
		adapter, router, err := NewCorruptGossipSubAdapter(ctx, logger, host, cfg, clusterChangeConsumer)
		for _, opt := range routerOpts {
			opt(router)
		}
		return adapter, err
	}
	return factory
}

// CorruptGossipSubConfigFactory returns a factory function that creates a new instance of the forked gossipsub config
// from github.com/yhassanzadeh13/go-libp2p-pubsub for the purpose of BFT testing and attack vector implementation.
func CorruptGossipSubConfigFactory(opts ...CorruptPubSubAdapterConfigOption) p2p.GossipSubAdapterConfigFunc {
	return func(base *p2p.BasePubSubAdapterConfig) p2p.PubSubAdapterConfig {
		return NewCorruptPubSubAdapterConfig(base, opts...)
	}
}

// CorruptGossipSubConfigFactoryWithInspector returns a factory function that creates a new instance of the forked gossipsub config
// from github.com/yhassanzadeh13/go-libp2p-pubsub for the purpose of BFT testing and attack vector implementation.
func CorruptGossipSubConfigFactoryWithInspector(inspector func(peer.ID, *corrupt.RPC) error) p2p.GossipSubAdapterConfigFunc {
	return func(base *p2p.BasePubSubAdapterConfig) p2p.PubSubAdapterConfig {
		return NewCorruptPubSubAdapterConfig(base, WithInspector(inspector))
	}
}

func overrideWithCorruptGossipSub(builder p2p.NodeBuilder, opts ...CorruptPubSubAdapterConfigOption) {
	factory := CorruptGossipSubFactory()
	builder.SetGossipSubFactory(factory, CorruptGossipSubConfigFactory(opts...))
}
