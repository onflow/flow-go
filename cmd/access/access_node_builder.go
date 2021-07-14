package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/network"
	jsoncodec "github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/network/validator"
)

// AccessNodeBuilder is initializes and runs an Access node either as a staked node or an unstaked node
// It is composed of the FlowNodeBuilder
type AccessNodeBuilder struct {
	cmd.NodeBuilder
	staked                  bool
	stakedAccessNodeIDHex   string
	unstakedNetworkBindAddr string
	unstakedNetwork         *p2p.Network
	unstakedMiddleware      *p2p.Middleware
}

func FlowAccessNode() *AccessNodeBuilder {
	return &AccessNodeBuilder{
		FlowNodeBuilder: cmd.FlowNode(flow.RoleAccess.String()),
	}
}

func (anb *AccessNodeBuilder) Initialize() *AccessNodeBuilder {

	anb.PrintBuildVersionDetails()

	anb.BaseFlags()

	anb.ParseAndPrintFlags()

	anb.validateParams()

	// for the staked access node, initialize the network used to communicate with the other staked flow nodes
	if anb.staked {
		anb.EnqueueNetworkInit()
	}

	// if an unstaked bind address is provided, initialize the network to communicate on the unstaked network
	if anb.unstakedNetworkBindAddr != cmd.NotSet {
		anb.EnqueueUnstakedNetworkInit()
	}

	anb.EnqueueMetricsServerInit()

	anb.RegisterBadgerMetrics()

	anb.EnqueueTracer()

	return anb
}

func (anb *AccessNodeBuilder) validateParams() {
	if anb.staked {
		return
	}

	// for an unstaked access node, the staked access node ID must be provided
	if strings.TrimSpace(anb.stakedAccessNodeIDHex) == "" {
		anb.Logger.Fatal().Msg("staked access node ID not specified")
	}

	// and also the unstaked bind address
	if anb.unstakedNetworkBindAddr == cmd.NotSet {
		anb.Logger.Fatal().Msg("unstaked bind address not set")
	}
}

func (anb *AccessNodeBuilder) EnqueueUnstakedNetworkInit() {
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

		libP2PNodeFactory, err := p2p.DefaultLibP2PNodeFactory(node.Logger.Level(zerolog.ErrorLevel),
			node.Me.NodeID(),
			anb.unstakedNetworkBindAddr,
			node.NetworkKey,
			node.RootBlock.ID().String(),
			p2p.DefaultMaxPubSubMsgSize,
			node.Metrics().Network,
			pingProvider)
		if err != nil {
			return nil, fmt.Errorf("could not generate libp2p node factory: %w", err)
		}

		peerUpdateInterval := time.Hour // pretty much not needed for the unstaked access node

		var msgValidators []network.MessageValidator
		if anb.staked {
			msgValidators = p2p.DefaultValidators(node.Logger(), node.Me.NodeID())
		} else {
			// for an unstaked node, use message sender validator but not target validator since the staked AN will
			// be broadcasting messages to ALL unstaked ANs without knowing their target IDs
			msgValidators = []network.MessageValidator{
				// filter out messages sent by this node itself
				validator.NewSenderValidator(node.Me.NodeID()),
				// but retain all the 1-k messages even if they are not intended for this node
			}
		}

		anb.unstakedMiddleware = p2p.NewMiddleware(node.Logger.Level(zerolog.ErrorLevel),
			libP2PNodeFactory,
			node.Me.NodeID(),
			node.Metrics().Network,
			node.RootBlock.ID().String(),
			peerUpdateInterval,
			p2p.DefaultUnicastTimeout,
			msgValidators...)

		participants, err := node.ProtocolState().Final().Identities(p2p.NetworkingSetFilter)
		if err != nil {
			return nil, fmt.Errorf("could not get network identities: %w", err)
		}

		// creates topology, topology manager, and subscription managers
		//
		// topology
		// subscription manager
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
			node.Me,
			anb.unstakedMiddleware,
			10e6,
			top,
			subscriptionManager,
			node.Metrics().Network)
		if err != nil {
			return nil, fmt.Errorf("could not initialize network: %w", err)
		}

		anb.unstakedNetwork = net

		// for an unstaked node, the staked network and middleware is set to the same as the unstaked network
		if !anb.staked {
			anb.Network = anb.unstakedNetwork
			anb.Middleware = anb.unstakedMiddleware
		}
		anb.Logger.Info().Msgf("unstaked network will run on address: %s", anb.unstakedNetworkBindAddr)
		return net, err
	})
}

func (anb *AccessNodeBuilder) initUnstakedLocal() func(node cmd.NodeBuilder) {
	return func(node cmd.NodeBuilder) {
		// for an unstaked node, set the identity here explicitly since it will not be found in the protocol state
		self := &flow.Identity{
			NodeID:        anb.NodeID,
			NetworkPubKey: anb.NetworkKey.PublicKey(),
			StakingPubKey: nil,             // no staking key needed for the unstaked node
			Role:          flow.RoleAccess, // unstaked node can only run as an access node
			Address:       anb.unstakedNetworkBindAddr,
		}

		me, err := local.New(self, nil)
		anb.MustNot(err).Msg("could not initialize local")
		anb.Me = me
	}
}
