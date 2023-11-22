package cmd

import (
	"fmt"
	"net"

	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/corruptnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/utils/logging"
)

// CorruptNetworkPort is the port number that gRPC server of the corrupt networking layer of the corrupted nodes is listening on.
const CorruptNetworkPort = "4300"

// CorruptedNodeBuilder creates a general flow node builder with corrupt network.
type CorruptedNodeBuilder struct {
	*cmd.FlowNodeBuilder
	TopicValidatorDisabled                bool
	WithPubSubMessageSigning              bool // libp2p option that enables message signing on the node
	WithPubSubStrictSignatureVerification bool // libp2p option that enforces message signature verification
}

func NewCorruptedNodeBuilder(role string) *CorruptedNodeBuilder {
	return &CorruptedNodeBuilder{
		FlowNodeBuilder:                       cmd.FlowNode(role),
		TopicValidatorDisabled:                true,
		WithPubSubMessageSigning:              true,
		WithPubSubStrictSignatureVerification: true,
	}
}

// LoadCorruptFlags load additional flags for corrupt nodes.
func (cnb *CorruptedNodeBuilder) LoadCorruptFlags() {
	cnb.FlowNodeBuilder.ExtraFlags(func(flags *pflag.FlagSet) {
		flags.BoolVar(&cnb.TopicValidatorDisabled, "topic-validator-disabled", true, "enable the libp2p topic validator for corrupt nodes")
		flags.BoolVar(&cnb.WithPubSubMessageSigning, "pubsub-message-signing", true, "enable pubsub message signing for corrupt nodes")
		flags.BoolVar(&cnb.WithPubSubStrictSignatureVerification, "pubsub-strict-sig-verification", true, "enable pubsub strict signature verification for corrupt nodes")
	})
}

func (cnb *CorruptedNodeBuilder) Initialize() error {

	// skip FlowNodeBuilder initialization if node role is access. This is because the AN builder uses
	// a slightly different build flow than the other node roles. Flags and components are initialized
	// in calls to anBuilder.ParseFlags & anBuilder.Initialize . Another call to FlowNodeBuilder.Initialize will
	// end up calling BaseFlags() and causing a flags redefined error.
	if cnb.NodeRole != flow.RoleAccess.String() {
		if err := cnb.FlowNodeBuilder.Initialize(); err != nil {
			return fmt.Errorf("could not initilized flow node builder: %w", err)
		}
	}

	cnb.enqueueNetworkingLayer() // initializes corrupted networking layer.

	return nil
}

func (cnb *CorruptedNodeBuilder) enqueueNetworkingLayer() {
	cnb.FlowNodeBuilder.OverrideComponent(cmd.LibP2PNodeComponent, func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		myAddr := cnb.FlowNodeBuilder.NodeConfig.Me.Address()
		if cnb.FlowNodeBuilder.BaseConfig.BindAddr != cmd.NotSet {
			myAddr = cnb.FlowNodeBuilder.BaseConfig.BindAddr
		}

		uniCfg := &p2pconfig.UnicastConfig{
			RateLimiterDistributor: cnb.UnicastRateLimiterDistributor,
			UnicastConfig:          cnb.FlowConfig.NetworkConfig.UnicastConfig,
		}

		connGaterCfg := &p2pconfig.ConnectionGaterConfig{
			InterceptPeerDialFilters: []p2p.PeerFilter{}, // disable connection gater onInterceptPeerDialFilters
			InterceptSecuredFilters:  []p2p.PeerFilter{}, // disable connection gater onInterceptSecuredFilters
		}

		peerManagerCfg := &p2pconfig.PeerManagerConfig{
			ConnectionPruning: cnb.FlowConfig.NetworkConfig.NetworkConnectionPruning,
			UpdateInterval:    cnb.FlowConfig.NetworkConfig.PeerUpdateInterval,
			ConnectorFactory:  connection.DefaultLibp2pBackoffConnectorFactory(),
		}

		// create default libp2p factory if corrupt node should enable the topic validator
		corruptLibp2pNode, err := corruptlibp2p.InitCorruptLibp2pNode(cnb.Logger,
			cnb.RootChainID,
			myAddr,
			cnb.NetworkKey,
			cnb.SporkID,
			cnb.IdentityProvider,
			cnb.Metrics.Network,
			cnb.Resolver,
			cnb.BaseConfig.NodeRole,
			connGaterCfg, // run peer manager with the specified interval and let it also prune connections
			peerManagerCfg,
			uniCfg,
			cnb.FlowConfig.NetworkConfig,
			&p2p.DisallowListCacheConfig{
				MaxSize: cnb.FlowConfig.NetworkConfig.DisallowListNotificationCacheSize,
				Metrics: metrics.DisallowListCacheMetricsFactory(cnb.HeroCacheMetricsFactory(), network.PrivateNetwork),
			},
			cnb.TopicValidatorDisabled,
			cnb.WithPubSubMessageSigning,
			cnb.WithPubSubStrictSignatureVerification)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p node: %w", err)
		}
		cnb.LibP2PNode = corruptLibp2pNode
		cnb.Logger.Info().
			Hex("node_id", logging.ID(cnb.NodeID)).
			Str("address", myAddr).
			Bool("topic_validator_disabled", cnb.TopicValidatorDisabled).
			Msg("corrupted libp2p node initialized")

		return corruptLibp2pNode, nil
	})
	cnb.FlowNodeBuilder.OverrideComponent(cmd.NetworkComponent, func(node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		myAddr := cnb.FlowNodeBuilder.NodeConfig.Me.Address()
		if cnb.FlowNodeBuilder.BaseConfig.BindAddr != cmd.NotSet {
			myAddr = cnb.FlowNodeBuilder.BaseConfig.BindAddr
		}

		host, _, err := net.SplitHostPort(myAddr)
		if err != nil {
			return nil, fmt.Errorf("could not extract host address: %w", err)
		}

		address := net.JoinHostPort(host, CorruptNetworkPort)
		ccf := corruptnet.NewCorruptConduitFactory(cnb.FlowNodeBuilder.Logger, cnb.FlowNodeBuilder.RootChainID)

		cnb.Logger.Info().Hex("node_id", logging.ID(cnb.NodeID)).Msg("corrupted conduit factory initiated")

		flowNetwork, err := cnb.FlowNodeBuilder.InitFlowNetworkWithConduitFactory(node, ccf, ratelimit.NoopRateLimiters(), []p2p.PeerFilter{})
		if err != nil {
			return nil, fmt.Errorf("could not initiate flow network: %w", err)
		}

		// initializes corruptible network that acts as a wrapper around the original flow network of the node, hence
		// allowing a remote attacker to control the ingress and egress traffic of the node.
		corruptibleNetwork, err := corruptnet.NewCorruptNetwork(cnb.Logger, cnb.RootChainID, address, cnb.Me, cnb.CodecFactory(), flowNetwork, ccf)
		if err != nil {
			return nil, fmt.Errorf("could not create corruptible network: %w", err)
		}
		cnb.Logger.Info().Hex("node_id", logging.ID(cnb.NodeID)).Str("address", address).Msg("corruptible network initiated")

		// override the original flow network with the corruptible network.
		cnb.EngineRegistry = corruptibleNetwork

		return corruptibleNetwork, nil
	})
}
