package cmd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/insecure/corruptnet"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/utils/logging"
)

// CorruptNetworkPort is the port number that gRPC server of the corrupt networking layer of the corrupted nodes is listening on.
const CorruptNetworkPort = 4300

// CorruptedNodeBuilder creates a general flow node builder with corrupt network.
type CorruptedNodeBuilder struct {
	*cmd.FlowNodeBuilder
}

func NewCorruptedNodeBuilder(role string) *CorruptedNodeBuilder {
	return &CorruptedNodeBuilder{
		FlowNodeBuilder: cmd.FlowNode(role),
	}
}

func (cnb *CorruptedNodeBuilder) Initialize() error {
	if err := cnb.FlowNodeBuilder.Initialize(); err != nil {
		return fmt.Errorf("could not initilized flow node builder: %w", err)
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

		libP2PNodeFactory := corruptnet.NewCorruptLibP2PNodeFactory(
			cnb.Logger,
			cnb.RootChainID,
			myAddr,
			cnb.NetworkKey,
			cnb.SporkID,
			cnb.IdentityProvider,
			cnb.Metrics.Network,
			cnb.Resolver,
			cnb.PeerScoringEnabled,
			cnb.BaseConfig.NodeRole,
			[]p2p.PeerFilter{}, // disable connection gater onInterceptPeerDialFilters
			[]p2p.PeerFilter{}, // disable connection gater onInterceptSecuredFilters
			// run peer manager with the specified interval and let it also prune connections
			cnb.NetworkConnectionPruning,
			cnb.PeerUpdateInterval,
		)

		libp2pNode, err := libP2PNodeFactory()
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p node: %w", err)
		}
		cnb.LibP2PNode = libp2pNode
		cnb.Logger.Info().Hex("node_id", logging.ID(cnb.NodeID)).Str("address", myAddr).Msg("corrupted libp2p node initialized")
		return libp2pNode, nil
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

		address := net.JoinHostPort(host, strconv.Itoa(CorruptNetworkPort))
		ccf := corruptnet.NewCorruptConduitFactory(cnb.FlowNodeBuilder.Logger, cnb.FlowNodeBuilder.RootChainID)

		cnb.Logger.Info().Hex("node_id", logging.ID(cnb.NodeID)).Msg("corrupted conduit factory initiated")

		flowNetwork, err := cnb.FlowNodeBuilder.InitFlowNetworkWithConduitFactory(node, ccf, ratelimit.NoopRateLimiters(), []p2p.PeerFilter{})
		if err != nil {
			return nil, fmt.Errorf("could not initiate flow network: %w", err)
		}

		// initializes corruptible network that acts as a wrapper around the original flow network of the node, hence
		// allowing a remote attacker to control the ingress and egress traffic of the node.
		corruptibleNetwork, err := corruptnet.NewCorruptNetwork(
			cnb.Logger,
			cnb.RootChainID,
			address,
			cnb.Me,
			cnb.CodecFactory(),
			flowNetwork,
			ccf)
		if err != nil {
			return nil, fmt.Errorf("could not create corruptible network: %w", err)
		}
		cnb.Logger.Info().Hex("node_id", logging.ID(cnb.NodeID)).Str("address", address).Msg("corruptible network initiated")

		// override the original flow network with the corruptible network.
		cnb.Network = corruptibleNetwork
		return corruptibleNetwork, nil
	})
}
