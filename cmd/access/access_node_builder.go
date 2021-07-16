package main

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	jsoncodec "github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/p2p"
)

// AccessNodeBuilder extends cmd.NodeBuilder and declares additional functions needed to bootstrap an Access node
type AccessNodeBuilder interface {
	cmd.NodeBuilder

	// IsStaked returns True is this is a staked Access Node, False otherwise
	IsStaked() bool
}

// FlowAccessNodeBuilder provides the common functionality needed to bootstrap a Flow staked and unstaked access node
// It is composed of the FlowNodeBuilder
type FlowAccessNodeBuilder struct {
	*cmd.FlowNodeBuilder
	staked                  bool
	stakedAccessNodeIDHex   string
	unstakedNetworkBindAddr string
	unstakedNetwork         *p2p.Network
	unstakedMiddleware      *p2p.Middleware
}

func FlowAccessNode() *FlowAccessNodeBuilder {
	return &FlowAccessNodeBuilder{
		FlowNodeBuilder: cmd.FlowNode(flow.RoleAccess.String()),
	}
}
func (anb *FlowAccessNodeBuilder) IsStaked() bool {
	return anb.staked
}

func (anb *FlowAccessNodeBuilder) parseFlags() {

	anb.BaseFlags()

	anb.ParseAndPrintFlags()
}

func (anb *FlowAccessNodeBuilder) initLibP2PFactory(nodeID flow.Identifier,
	networkMetrics module.NetworkMetrics,
	networkKey crypto.PrivateKey) (p2p.LibP2PFactoryFunc, error) {

	// setup the Ping provider to return the software version and the sealed block height
	pingProvider := p2p.PingInfoProviderImpl{
		SoftwareVersionFun: func() string {
			return build.Semver()
		},
		SealedBlockHeightFun: func() (uint64, error) {
			head, err := anb.ProtocolState().Sealed().Head()
			if err != nil {
				return 0, err
			}
			return head.Height, nil
		},
	}

	libP2PNodeFactory, err := p2p.DefaultLibP2PNodeFactory(anb.Logger(),
		nodeID,
		anb.unstakedNetworkBindAddr,
		networkKey,
		anb.RootBlock().ID().String(),
		p2p.DefaultMaxPubSubMsgSize,
		networkMetrics,
		pingProvider)
	if err != nil {
		return nil, fmt.Errorf("could not generate libp2p node factory: %w", err)
	}

	return libP2PNodeFactory, nil
}

func (anb *FlowAccessNodeBuilder) initMiddleware(nodeID flow.Identifier,
	networkMetrics module.NetworkMetrics,
	factoryFunc p2p.LibP2PFactoryFunc,
	peerUpdateInterval time.Duration,
	validators ...network.MessageValidator) *p2p.Middleware {
	anb.unstakedMiddleware = p2p.NewMiddleware(anb.Logger(),
		factoryFunc,
		nodeID,
		networkMetrics,
		anb.RootBlock().ID().String(),
		peerUpdateInterval,
		p2p.DefaultUnicastTimeout,
		validators...)
	return anb.unstakedMiddleware
}

func (anb *FlowAccessNodeBuilder) initNetwork(nodeID module.Local,
	networkMetrics module.NetworkMetrics,
	middleware *p2p.Middleware,
	participants flow.IdentityList,
	topology network.Topology) (*p2p.Network, error) {

	codec := jsoncodec.NewCodec()

	subscriptionManager := p2p.NewChannelSubscriptionManager(middleware)

	// creates network instance
	net, err := p2p.NewNetwork(anb.Logger(),
		codec,
		participants,
		nodeID,
		anb.unstakedMiddleware,
		p2p.DefaultCacheSize,
		topology,
		subscriptionManager,
		networkMetrics)
	if err != nil {
		return nil, fmt.Errorf("could not initialize network: %w", err)
	}

	return net, nil
}
