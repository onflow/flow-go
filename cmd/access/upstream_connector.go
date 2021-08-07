package main

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/network/p2p"
)

// upstreamConnector connects the unstaked AN with an upstream staked AN
type upstreamConnector struct {
	lm               *lifecycle.LifecycleManager
	stakedANIdentity *flow.Identity
	logger           zerolog.Logger
	unstakedNode     *p2p.Node
	cancel           context.CancelFunc
}

func newUpstreamConnector(stakedAN *flow.Identity, unstakedNode *p2p.Node, logger zerolog.Logger) *upstreamConnector {
	return &upstreamConnector{
		lm:               lifecycle.NewLifecycleManager(),
		stakedANIdentity: stakedAN,
		unstakedNode:     unstakedNode,
		logger:           logger,
	}
}
func (connector *upstreamConnector) Ready() <-chan struct{} {
	connector.lm.OnStart(func() {
		const maxAttempts = 3
		ctx, cancel := context.WithCancel(context.Background())
		connector.cancel = cancel
		log := connector.logger.With().Str("staked_an", connector.stakedANIdentity.String()).Logger()
		for i := 1; i <= maxAttempts; i++ {
			err := connector.unstakedNode.AddPeer(ctx, *connector.stakedANIdentity)
			if err == nil {
				log.Info().Msg("connected to upstream access node")
				return
			}
			log.Error().Int("attempt", i).Int("max_attempts", maxAttempts).Msg("failed to connected to the staked access node")
			time.Sleep(time.Second)
		}
		// log fatal as there is no point continuing further if the unstaked AN cannot connect to the staked AN
		log.Fatal().Msg("failed to connected to the staked access node, " +
			"please ensure the node ID, network address and public key of the staked access node are correct " +
			"and that the staked access node is running.")
	})
	return connector.lm.Started()
}

func (connector *upstreamConnector) Done() <-chan struct{} {
	connector.lm.OnStop(func() {
		if connector.cancel != nil {
			connector.cancel()
		}
	})
	return connector.lm.Stopped()
}
