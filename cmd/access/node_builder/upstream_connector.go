package node_builder

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/network/p2p"
)

// upstreamConnector tries to connect the unstaked AN with atleast one of the configured bootstrap access nodes
type upstreamConnector struct {
	lm                  *lifecycle.LifecycleManager
	bootstrapIdentities flow.IdentityList
	logger              zerolog.Logger
	unstakedNode        *p2p.Node
	cancel              context.CancelFunc
	retryInitialTimeout time.Duration
	maxRetries          uint64
}

func newUpstreamConnector(bootstrapIdentities flow.IdentityList, unstakedNode *p2p.Node, logger zerolog.Logger) *upstreamConnector {
	return &upstreamConnector{
		lm:                  lifecycle.NewLifecycleManager(),
		bootstrapIdentities: bootstrapIdentities,
		unstakedNode:        unstakedNode,
		logger:              logger,
		retryInitialTimeout: time.Second,
		maxRetries:          5,
	}
}
func (connector *upstreamConnector) Ready() <-chan struct{} {
	connector.lm.OnStart(func() {
		// eventually, context will be passed in to Start method: https://github.com/dapperlabs/flow-go/issues/5730
		ctx, cancel := context.WithCancel(context.TODO())
		connector.cancel = cancel

		workerCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		success := atomic.NewBool(false)
		var wg sync.WaitGroup

		// spawn a connect worker for each bootstrap node
		for _, b := range connector.bootstrapIdentities {
			id := *b
			wg.Add(1)
			go func() {
				defer wg.Done()
				lg := connector.logger.With().Str("bootstrap_node", id.NodeID.String()).Logger()

				fibRetry, err := retry.NewFibonacci(connector.retryInitialTimeout)
				if err != nil {
					lg.Err(err).Msg("cannot create retry mechanism")
					return
				}
				cappedFibRetry := retry.WithMaxRetries(connector.maxRetries, fibRetry)

				if err = retry.Do(workerCtx, cappedFibRetry, func(ctx context.Context) error {
					return retry.RetryableError(connector.connect(ctx, id))
				}); err != nil {
					lg.Err(err).Msg("failed to connect")
				} else {
					lg.Info().Msg("successfully connected to bootstrap node")
					success.Store(true)
				}
			}()
		}

		wg.Wait()

		if !success.Load() {
			// log fatal as there is no point continuing further, the unstaked AN cannot connect to any of the bootstrap peers
			connector.logger.Fatal().
				Msg("Failed to connect to a bootstrap node. " +
					"Please ensure the network address and public key of the bootstrap access node are correct " +
					"and that the node is running and reachable.")
		}
	})
	return connector.lm.Started()
}

// connect is run to connect to an boostrap peer
func (connector *upstreamConnector) connect(ctx context.Context, bootstrapPeer flow.Identity) error {

	select {
	// check for a cancelled/expired context
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	peerAddrInfo, err := p2p.PeerAddressInfo(bootstrapPeer)

	if err != nil {
		return err
	}

	// try and connect to the bootstrap server
	return connector.unstakedNode.AddPeer(ctx, peerAddrInfo)
}

func (connector *upstreamConnector) Done() <-chan struct{} {
	connector.lm.OnStop(func() {
		// this function will only be executed if connector.lm.OnStart was previously called,
		// in which case connector.cancel != nil
		connector.cancel()
	})
	return connector.lm.Stopped()
}
