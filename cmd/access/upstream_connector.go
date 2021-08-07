package main

import (
	"context"
	"time"

	"github.com/hashicorp/go-multierror"
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
	ctx              context.Context
	cancel           context.CancelFunc
}

func newUpstreamConnector(ctx context.Context, stakedAN *flow.Identity, unstakedNode *p2p.Node, logger zerolog.Logger) *upstreamConnector {

	ctx, cancel := context.WithCancel(ctx)
	return &upstreamConnector{
		lm:               lifecycle.NewLifecycleManager(),
		stakedANIdentity: stakedAN,
		unstakedNode:     unstakedNode,
		logger:           logger,
		ctx:              ctx,
		cancel:           cancel,
	}
}
func (connector *upstreamConnector) Ready() <-chan struct{} {
	connector.lm.OnStart(func() {
		const maxAttempts = 3
		log := connector.logger.With().
			Str("staked_an", connector.stakedANIdentity.String()).
			Int("max_attempts", maxAttempts).
			Logger()

		var errors *multierror.Error
		for i := 1; i <= maxAttempts; i++ {

			select {
			// check for a cancelled/expired context
			case <-connector.ctx.Done():
				return
			default:
			}

			err := connector.unstakedNode.AddPeer(connector.ctx, *connector.stakedANIdentity)
			if err == nil {
				log.Info().Int("attempt", i).Msg("successfully connected to upstream access node")
				return
			}

			log.Error().Err(err).Int("attempt", i).Msg("failed to connected to the staked access node")
			errors = multierror.Append(errors, err)

			time.Sleep(time.Second)
		}
		// if we made it till here, means there was atleast one error
		err := errors.ErrorOrNil()

		// log fatal as there is no point continuing further if the unstaked AN cannot connect to the staked AN
		log.Fatal().Err(err).
			Msg("Failed to connect to the staked access node. " +
				"Please ensure the node ID, network address and public key of the staked access node are correct " +
				"and that the staked access node is running and reachable.")
	})
	return connector.lm.Started()
}

func (connector *upstreamConnector) Done() <-chan struct{} {
	connector.lm.OnStop(func() {
		connector.cancel()
	})
	return connector.lm.Stopped()
}
