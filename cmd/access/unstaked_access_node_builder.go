package main

import (
	"strings"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
)

type UnstakedAccessNodeBuilder struct {
	*FlowAccessNodeBuilder
}

func UnstakedAccessNode(anb *FlowAccessNodeBuilder) *UnstakedAccessNodeBuilder {
	return &UnstakedAccessNodeBuilder{
		FlowAccessNodeBuilder: anb,
	}
}

func (uanb *UnstakedAccessNodeBuilder) Initialize() cmd.NodeBuilder {

	uanb.validateParams()

	uanb.EnqueueUnstakedNetworkInit()

	uanb.EnqueueMetricsServerInit()

	uanb.RegisterBadgerMetrics()

	uanb.EnqueueTracer()

	uanb.PreInit(uanb.initUnstakedLocal())

	return uanb
}

func (uanb *UnstakedAccessNodeBuilder) validateParams() {

	logger := uanb.Logger()
	// for an unstaked access node, the staked access node ID must be provided
	if strings.TrimSpace(uanb.stakedAccessNodeIDHex) == "" {
		logger.Fatal().Msg("staked access node ID not specified")
	}

	// and also the unstaked bind address
	if uanb.unstakedNetworkBindAddr == cmd.NotSet {
		logger.Fatal().Msg("unstaked bind address not set")
	}
}

func (uanb *UnstakedAccessNodeBuilder) initUnstakedLocal() func(node cmd.NodeBuilder) {
	return func(node cmd.NodeBuilder) {
		// for an unstaked node, set the identity here explicitly since it will not be found in the protocol state
		self := &flow.Identity{
			NodeID:        node.NodeID(),
			NetworkPubKey: node.NetworkKey().PublicKey(),
			StakingPubKey: nil,             // no staking key needed for the unstaked node
			Role:          flow.RoleAccess, // unstaked node can only run as an access node
			Address:       uanb.unstakedNetworkBindAddr,
		}

		me, err := local.New(self, nil)
		node.MustNot(err).Msg("could not initialize local")
		node.SetMe(me)
	}
}
