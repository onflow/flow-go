package main

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	jsoncodec "github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/network/validator"
)

// AccessNodeBuilder extends cmd.NodeBuilder and declares additional functions needed to bootstrap an Access node
type AccessNodeBuilder interface {
	cmd.NodeBuilder
	EnqueueUnstakedNetworkInit()
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

func (anb *FlowAccessNodeBuilder) Initialize() cmd.NodeBuilder {

	anb.PrintBuildVersionDetails()

	anb.BaseFlags()

	anb.ParseAndPrintFlags()

	return anb
}

func (anb *FlowAccessNodeBuilder) EnqueueUnstakedNetworkInit() {
	anb.Component("unstaked network", func(node cmd.NodeBuilder) (module.ReadyDoneAware, error) {
		codec := jsoncodec.NewCodec()

		// setup the Ping provider to return the software version and the finalized block height
		pingProvider := p2p.PingInfoProviderImpl{
			SoftwareVersionFun: func() string {
				return build.Semver()
			},
			SealedBlockHeightFun: func() (uint64, error) {
				head, err := node.ProtocolState().Sealed().Head()
				if err != nil {
					return 0, err
				}
				return head.Height, nil
			},
		}

		libP2PNodeFactory, err := p2p.DefaultLibP2PNodeFactory(node.Logger(),
			node.Me().NodeID(),
			anb.unstakedNetworkBindAddr,
			node.NetworkKey(),
			node.RootBlock().ID().String(),
			p2p.DefaultMaxPubSubMsgSize,
			node.Metrics().Network,
			pingProvider)
		if err != nil {
			return nil, fmt.Errorf("could not generate libp2p node factory: %w", err)
		}

		peerUpdateInterval := time.Hour // pretty much not needed for the unstaked access node

		var msgValidators []network.MessageValidator
		if anb.staked {
			msgValidators = p2p.DefaultValidators(node.Logger(), node.Me().NodeID())
		} else {
			// for an unstaked node, use message sender validator but not target validator since the staked AN will
			// be broadcasting messages to ALL unstaked ANs without knowing their target IDs
			msgValidators = []network.MessageValidator{
				// filter out messages sent by this node itself
				validator.NewSenderValidator(node.Me().NodeID()),
				// but retain all the 1-k messages even if they are not intended for this node
			}
		}

		anb.unstakedMiddleware = p2p.NewMiddleware(node.Logger().Level(zerolog.ErrorLevel),
			libP2PNodeFactory,
			node.Me().NodeID(),
			node.Metrics().Network,
			node.RootBlock().ID().String(),
			peerUpdateInterval,
			p2p.DefaultUnicastTimeout,
			msgValidators...)

		participants, err := node.ProtocolState().Final().Identities(p2p.NetworkingSetFilter)
		if err != nil {
			return nil, fmt.Errorf("could not get network identities: %w", err)
		}

		subscriptionManager := p2p.NewChannelSubscriptionManager(anb.unstakedMiddleware)
		var top network.Topology
		if anb.staked {
			top = topology.EmptyListTopology{}
		} else {
			upstreamANIdentifier, err := flow.HexStringToIdentifier(anb.stakedAccessNodeIDHex)
			if err != nil {
				return nil, fmt.Errorf("failed to convert node id string %s to Flow Identifier: %v", anb.stakedAccessNodeIDHex, err)
			}
			top = topology.NewFixedListTopology(upstreamANIdentifier)
		}

		// creates network instance
		net, err := p2p.NewNetwork(node.Logger(),
			codec,
			participants,
			node.Me(),
			anb.unstakedMiddleware,
			p2p.DefaultCacheSize,
			top,
			subscriptionManager,
			node.Metrics().Network)
		if err != nil {
			return nil, fmt.Errorf("could not initialize network: %w", err)
		}

		anb.unstakedNetwork = net

		// for an unstaked node, the staked network and middleware is set to the same as the unstaked network
		if !anb.staked {
			anb.SetNetwork(anb.unstakedNetwork)
			anb.SetMiddleware(anb.unstakedMiddleware)
		}
		logger := anb.Logger()
		logger.Info().Msgf("unstaked network will run on address: %s", anb.unstakedNetworkBindAddr)
		return net, err
	})
}
